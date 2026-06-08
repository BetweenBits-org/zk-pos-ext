// Package tierpolicy is the engine's single, audit-aligned source of
// truth for two operations that the PoR engine and its external callers
// (operator console, snapshot import tooling, zk-prover DB import) must
// perform byte-for-byte identically:
//
//  1. BuildTierCurve derives the precomputed_value cumulative column of
//     a piecewise-linear haircut curve from its authoritative
//     (boundary_value, ratio) inputs, using the exact integer recipe the
//     audited T3/T4 circuits enforce in-circuit
//     (core/solvency/t{3,4}_*/circuit/utils.go
//     generateRapidArithmeticForCollateral). Callers MUST NOT re-derive
//     this recipe by hand — a one-bit divergence produces a
//     witness/proof that fails late or, worse, drifts from the audited
//     circuit semantics.
//
//  2. PolicyCommitment is a canonical, capacity-independent digest over
//     an operator's risk policy (T2 per-asset haircut_bp; T3/T4
//     per-asset, per-pool tier curves). An operator pins this digest in
//     profile.toml; the engine recomputes it from the snapshot and
//     rejects any snapshot whose policy does not reproduce the pinned
//     value (fail-closed authorization). The digest covers only the
//     authoritative policy inputs (boundary_value, ratio, haircut_bp) —
//     never the derived precomputed_value, and never the volatile
//     per-snapshot totals — so the operator can compute it from the
//     policy alone, without knowing the deployment asset capacity.
//
// Keeping both operations here removes the audit-drift risk of a caller
// re-implementing the cumulative recipe or the digest encoding.
package tierpolicy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"

	corespec "github.com/BetweenBits-org/zk-pos-ext/core/spec"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// PercentMultiplier is the haircut-ratio denominator: a Ratio of N means
// a haircut weight of N/100. Mirrors the audited circuit's
// PercentageMultiplierFr (= 100).
const PercentMultiplier = 100

// MaxTiers is the maximum number of tiers permitted in one pool's curve.
// Mirrors corespec.TierCount (the per-pool tier slot count the circuits
// pad to). Building a curve with more tiers than this can never be
// committed by the engine, so BuildTierCurve rejects it.
const MaxTiers = corespec.TierCount

// MaxBoundaryValue is the inclusive upper bound the audited circuit
// range-checks every boundary_value against (2^118 — the
// MaxTierBoundaryValueFr sentinel reused as the reserved-tier boundary).
var MaxBoundaryValue = new(big.Int).Lsh(big.NewInt(1), 118)

// Tier is one authoritative (boundary, ratio) input pair of a
// piecewise-linear haircut curve. Boundary is the tier's inclusive upper
// boundary in balance-scaled integer units; Ratio is the haircut
// percentage in [0, 100].
type Tier struct {
	Boundary *big.Int
	Ratio    uint8
}

// TierRatio is one built tier: the authoritative (Boundary, Ratio) plus
// the derived Precomputed cumulative haircut value at Boundary. Field
// names mirror core/solvency/t{3,4}_*/spec.TierRatio so callers can map
// across without ambiguity.
type TierRatio struct {
	Boundary    *big.Int
	Ratio       uint8
	Precomputed *big.Int
}

// BuildTierCurve derives the precomputed_value column for a single
// pool's tier curve from its (boundary, ratio) inputs, reproducing the
// audited circuit recipe exactly:
//
//	precomputed[0] = floor(boundary[0] * ratio[0] / 100)
//	precomputed[i] = precomputed[i-1] +
//	                 floor((boundary[i] - boundary[i-1]) * ratio[i] / 100)
//
// so precomputed[i] is the cumulative haircut value at boundary[i]. The
// returned slice carries freshly allocated big.Ints; the input is not
// mutated.
//
// It rejects any curve the audited circuit's range checks would reject —
// returning a descriptive error rather than letting the divergence
// surface later as a failed proof:
//   - empty curve, or more than MaxTiers tiers
//   - a nil or negative boundary, or a boundary exceeding MaxBoundaryValue
//   - a ratio greater than 100
//   - boundaries not strictly increasing
func BuildTierCurve(tiers []Tier) ([]TierRatio, error) {
	if len(tiers) == 0 {
		return nil, fmt.Errorf("tierpolicy: empty tier curve")
	}
	if len(tiers) > MaxTiers {
		return nil, fmt.Errorf("tierpolicy: %d tiers exceeds MaxTiers (%d)", len(tiers), MaxTiers)
	}
	out := make([]TierRatio, len(tiers))
	divisor := big.NewInt(PercentMultiplier)
	prevBoundary := big.NewInt(0)
	prevPrecomp := big.NewInt(0)
	for i, t := range tiers {
		if t.Boundary == nil {
			return nil, fmt.Errorf("tierpolicy: tier %d has nil boundary", i)
		}
		if t.Boundary.Sign() < 0 {
			return nil, fmt.Errorf("tierpolicy: tier %d boundary is negative", i)
		}
		if t.Boundary.Cmp(MaxBoundaryValue) > 0 {
			return nil, fmt.Errorf("tierpolicy: tier %d boundary exceeds MaxBoundaryValue (2^118)", i)
		}
		if t.Ratio > PercentMultiplier {
			return nil, fmt.Errorf("tierpolicy: tier %d ratio %d exceeds 100", i, t.Ratio)
		}
		if i > 0 && t.Boundary.Cmp(prevBoundary) <= 0 {
			return nil, fmt.Errorf("tierpolicy: tier %d boundary not strictly increasing", i)
		}
		// current = floor((boundary - prevBoundary) * ratio / 100)
		current := new(big.Int).Sub(t.Boundary, prevBoundary)
		current.Mul(current, big.NewInt(int64(t.Ratio)))
		current.Div(current, divisor)
		precomp := new(big.Int).Add(prevPrecomp, current)
		out[i] = TierRatio{
			Boundary:    new(big.Int).Set(t.Boundary),
			Ratio:       t.Ratio,
			Precomputed: precomp,
		}
		prevBoundary = t.Boundary
		prevPrecomp = precomp
	}
	return out, nil
}

// AssetPolicy is one asset's risk policy entry. The fields used depend
// on the model:
//   - T2: Haircut holds the static haircut in basis points (0..10000);
//     Pools MUST be empty.
//   - T3: Pools holds exactly one tier curve; Haircut is unused.
//   - T4: Pools holds exactly three tier curves, in the canonical pool
//     order loan, margin, portfolio_margin; Haircut is unused.
type AssetPolicy struct {
	AssetIndex uint16
	Haircut    uint16
	Pools      [][]Tier
}

// Policy is a complete operator risk policy for one deployment model.
// Asset order is irrelevant: PolicyCommitment canonicalises by
// AssetIndex ascending before hashing.
type Policy struct {
	Model  corespec.SolvencyModelID
	Assets []AssetPolicy
}

// poolsExpectedFor returns the exact number of tier-curve pools each
// asset must carry for the model, and false for a model that has no
// tier-pool policy (T2 uses Haircut, every other model is unsupported).
func poolsExpectedFor(model corespec.SolvencyModelID) (pools int, tiered bool, ok bool) {
	switch model {
	case corespec.T2StaticHaircutMargin:
		return 0, false, true
	case corespec.T3TieredHaircutMargin1Pool:
		return 1, true, true
	case corespec.T4TieredHaircutMargin3Pool:
		return 3, true, true
	default:
		return 0, false, false
	}
}

// modelTag returns the domain-separation tag absorbed first into the
// digest so that two models can never share a commitment.
func modelTag(model corespec.SolvencyModelID) int64 {
	switch model {
	case corespec.T2StaticHaircutMargin:
		return 2
	case corespec.T3TieredHaircutMargin1Pool:
		return 3
	case corespec.T4TieredHaircutMargin3Pool:
		return 4
	default:
		return 0
	}
}

// PolicyCommitment returns the canonical Poseidon/BN254 digest of an
// operator risk policy. The encoding is a domain-separated,
// length-prefixed sequence of field elements absorbed in this order:
//
//	modelTag, len(assets),
//	for each asset (ascending AssetIndex):
//	    AssetIndex,
//	    T2:        Haircut
//	    T3/T4:     len(pools), for each pool: len(tiers),
//	               for each tier: Boundary, Ratio
//
// Only authoritative inputs are absorbed — never the derived
// precomputed_value, never the per-snapshot totals — and reserved
// padding slots are excluded, so the digest is independent of the
// deployment asset capacity. The same policy always yields the same
// digest; any change to a haircut, a boundary, a ratio, the asset set,
// or the model changes it.
//
// It returns a descriptive error if the policy is malformed for its
// model: an unsupported model, a duplicate AssetIndex, a T2 asset
// carrying tier pools, a T3/T4 asset whose pool count is wrong, or a
// tier curve the audited circuit would reject (see BuildTierCurve).
func PolicyCommitment(p Policy) ([]byte, error) {
	wantPools, tiered, ok := poolsExpectedFor(p.Model)
	if !ok {
		return nil, fmt.Errorf("tierpolicy: model %q has no risk policy", p.Model)
	}
	if len(p.Assets) == 0 {
		return nil, fmt.Errorf("tierpolicy: policy has no assets")
	}

	assets := make([]AssetPolicy, len(p.Assets))
	copy(assets, p.Assets)
	sort.Slice(assets, func(i, j int) bool { return assets[i].AssetIndex < assets[j].AssetIndex })

	h := poseidon.NewPoseidon()
	absorb := func(v *big.Int) { h.Write(v.Bytes()) }
	absorbInt := func(v int64) { absorb(big.NewInt(v)) }

	absorbInt(modelTag(p.Model))
	absorbInt(int64(len(assets)))

	seen := make(map[uint16]bool, len(assets))
	for _, a := range assets {
		if seen[a.AssetIndex] {
			return nil, fmt.Errorf("tierpolicy: duplicate asset_index %d", a.AssetIndex)
		}
		seen[a.AssetIndex] = true
		absorbInt(int64(a.AssetIndex))

		if !tiered {
			if len(a.Pools) != 0 {
				return nil, fmt.Errorf("tierpolicy: asset %d: T2 policy must not carry tier pools", a.AssetIndex)
			}
			if a.Haircut > 10_000 {
				return nil, fmt.Errorf("tierpolicy: asset %d: haircut_bp %d exceeds 10000", a.AssetIndex, a.Haircut)
			}
			absorbInt(int64(a.Haircut))
			continue
		}

		if len(a.Pools) != wantPools {
			return nil, fmt.Errorf("tierpolicy: asset %d: got %d pools, model %q requires %d",
				a.AssetIndex, len(a.Pools), p.Model, wantPools)
		}
		absorbInt(int64(len(a.Pools)))
		for pi, pool := range a.Pools {
			// Validate non-empty curves via the audited recipe; rejects any
			// tier curve the circuit would reject. An empty pool is allowed —
			// it represents an asset with no curve in that pool (collateral
			// there earns no credit), matching the engine's all-reserved tier
			// padding. Only the authoritative (boundary, ratio) inputs are
			// absorbed; the built curve is discarded.
			if len(pool) > 0 {
				if _, err := BuildTierCurve(pool); err != nil {
					return nil, fmt.Errorf("tierpolicy: asset %d pool %d: %w", a.AssetIndex, pi, err)
				}
			}
			absorbInt(int64(len(pool)))
			for _, t := range pool {
				absorb(t.Boundary)
				absorbInt(int64(t.Ratio))
			}
		}
	}
	return h.Sum(nil), nil
}

// VerifyCommitment reports whether got equals the operator-pinned policy
// commitment expectedHex (a hex string, with or without a "0x" prefix).
// It is the comparison half of the fail-closed authorization path: a
// caller obtains the snapshot's actual digest from the engine (a
// SnapshotSource.PolicyCommitment) and checks it against the value the
// operator pinned in profile.toml. Returns a descriptive error if
// expectedHex is malformed or the digests differ; nil on a match.
func VerifyCommitment(got []byte, expectedHex string) error {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(expectedHex, "0x"), "0X")
	want, err := hex.DecodeString(trimmed)
	if err != nil {
		return fmt.Errorf("tierpolicy: malformed expected commitment %q: %w", expectedHex, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("tierpolicy: policy commitment mismatch: snapshot=%x pinned=%x", got, want)
	}
	return nil
}

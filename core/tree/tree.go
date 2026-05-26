// Package tree wraps the bnb-chain/zkbnb-smt sparse Merkle tree at
// the engine's standard depth (corespec.AccountTreeDepth) and the
// Poseidon-over-BN254 leaf hasher. Universal across solvency models —
// the SMT shape is part of the trusted-setup contract; changing depth
// or hasher invalidates published proofs.
//
// Used by the witness service (build) and userproof service
// (per-account inclusion proofs). Verifier-side off-circuit Merkle
// verification lives at zkpor/core/host — it does not need the full
// tree, only the path-hashing primitive.
package tree

import (
	"fmt"
	"hash"
	"time"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/core/spec"
	bsmt "github.com/bnb-chain/zkbnb-smt"
	"github.com/bnb-chain/zkbnb-smt/database"
	"github.com/bnb-chain/zkbnb-smt/database/memory"
	"github.com/bnb-chain/zkbnb-smt/database/redis"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon"
)

// EmptyAccountLeafHash is the canonical leaf value for an account slot
// that has never been written: Poseidon over BN254 of five zero field
// elements — (accountID, totalEquity, totalDebt, totalCollateral,
// assetsCommitment) all zero. Embedded as the SparseMerkleTree's
// nil-leaf so an untouched leaf hashes to the value the t4_tiered_haircut_margin_3pool
// circuit's EmptyAccountLeafNodeHash expects.
var EmptyAccountLeafHash []byte

func init() {
	zero := &fr.Element{0, 0, 0, 0}
	h := poseidon.Poseidon(zero, zero, zero, zero, zero).Bytes()
	EmptyAccountLeafHash = h[:]
}

// NewAccountTree returns a depth-AccountTreeDepth sparse Merkle tree
// rooted on EmptyAccountLeafHash and backed by either an in-memory or
// Redis database. driver ∈ {"memory", "redis"}; for "redis", addr is
// the redis endpoint string. Unknown drivers return an error rather
// than silently producing a misconfigured tree.
//
// Connection-pool parameters for the redis backend mirror the legacy
// src/utils.NewAccountTree defaults (10s dial/read/write, 5min idle,
// 500 connections, 5 retries with 8ms/512ms backoff bounds).
func NewAccountTree(driver, addr string) (bsmt.SparseMerkleTree, error) {
	hasher := bsmt.NewHasherPool(func() hash.Hash { return poseidon.NewPoseidon() })

	var (
		db  database.TreeDB
		err error
	)
	switch driver {
	case "memory":
		db = memory.NewMemoryDB()
	case "redis":
		opt := &redis.RedisConfig{}
		opt.Addr = addr
		opt.DialTimeout = 10 * time.Second
		opt.ReadTimeout = 10 * time.Second
		opt.WriteTimeout = 10 * time.Second
		opt.PoolTimeout = 15 * time.Second
		opt.IdleTimeout = 5 * time.Minute
		opt.PoolSize = 500
		opt.MaxRetries = 5
		opt.MinRetryBackoff = 8 * time.Millisecond
		opt.MaxRetryBackoff = 512 * time.Millisecond
		db, err = redis.New(opt)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown account tree driver %q (want \"memory\" or \"redis\")", driver)
	}

	return bsmt.NewBNBSparseMerkleTree(hasher, db, spec.AccountTreeDepth, EmptyAccountLeafHash)
}

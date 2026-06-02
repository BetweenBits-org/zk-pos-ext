package store

import (
	corehost "github.com/BetweenBits-org/zk-pos-ext/core/host"
)

// This file holds the MySQL/gorm adapter-wrappers that let the gorm-bound
// *WitnessStore, *ProofStore, and *UserProofStore satisfy the gorm-free
// core/host persistence ports (corehost.WitnessQueue / ProofStore /
// UserProofStore). Each wrapper delegates to the inner store and maps
// between the port DTO and the gorm row field-by-field, copying every
// string payload (WitnessData, the proof JSON strings, the user-proof
// payloads) VERBATIM — no re-base64, trim, or re-marshal. The inner
// store.ErrNotFound sentinel is remapped to corehost.ErrNotFound on the
// Get*/Claim* paths so core callers can probe with corehost.IsNotFound
// without importing this package or gorm.
//
// The wrappers are NEW types, not extra methods on the inner stores: Go
// has no overloading, and a port-shaped Create([]DTO) would collide with
// the existing store Create([]row). The old store API stays intact.
//
// Import direction is one-way (store -> corehost). corehost never imports
// store or gorm.

// Compile-time assertions that each adapter satisfies its port.
var (
	_ corehost.WitnessQueue   = (*WitnessQueueAdapter)(nil)
	_ corehost.ProofStore     = (*ProofStoreAdapter)(nil)
	_ corehost.UserProofStore = (*UserProofStoreAdapter)(nil)
)

// notFound remaps the inner store's empty-result sentinel
// (store.ErrNotFound) to the port-owned corehost.ErrNotFound, leaving any
// other (real backing-store failure) error untouched. Adapters route
// every Get*/Claim* error through this so callers can probe absence with
// corehost.IsNotFound.
func notFound(err error) error {
	if IsNotFound(err) {
		return corehost.ErrNotFound
	}
	return err
}

// --- witness queue -------------------------------------------------------

// WitnessQueueAdapter adapts a *WitnessStore to the corehost.WitnessQueue
// port. It owns no state beyond the inner store; mapping is a verbatim
// field copy in both directions.
type WitnessQueueAdapter struct {
	inner *WitnessStore
}

// NewWitnessQueueAdapter wraps the given *WitnessStore as a
// corehost.WitnessQueue.
func NewWitnessQueueAdapter(inner *WitnessStore) *WitnessQueueAdapter {
	return &WitnessQueueAdapter{inner: inner}
}

// batchWitnessToDTO projects a gorm BatchWitness row onto the gorm-free
// port DTO. WitnessData is copied verbatim.
func batchWitnessToDTO(row *BatchWitness) corehost.BatchWitnessDTO {
	return corehost.BatchWitnessDTO{
		Height:      row.Height,
		WitnessData: row.WitnessData,
		Status:      row.Status,
	}
}

// batchWitnessFromDTO lifts a port DTO into a gorm BatchWitness row (with a
// zero gorm.Model — the backing fills in ID/timestamps). WitnessData is
// copied verbatim.
func batchWitnessFromDTO(dto corehost.BatchWitnessDTO) BatchWitness {
	return BatchWitness{
		Height:      dto.Height,
		WitnessData: dto.WitnessData,
		Status:      dto.Status,
	}
}

// EnsureSchema delegates to the inner store's CreateTable (gorm
// auto-migrate). Idempotent.
func (a *WitnessQueueAdapter) EnsureSchema() error {
	return a.inner.CreateTable()
}

// Create maps each DTO to a gorm row verbatim and inserts the batch via
// the inner store.
func (a *WitnessQueueAdapter) Create(rows []corehost.BatchWitnessDTO) error {
	out := make([]BatchWitness, len(rows))
	for i := range rows {
		out[i] = batchWitnessFromDTO(rows[i])
	}
	return a.inner.Create(out)
}

// ClaimOldestByStatus delegates to the inner store's transactional claim
// and projects the claimed row onto a DTO. An empty queue surfaces as
// corehost.ErrNotFound.
func (a *WitnessQueueAdapter) ClaimOldestByStatus(from, to int64) (*corehost.BatchWitnessDTO, error) {
	row, err := a.inner.ClaimOldestByStatus(from, to)
	if err != nil {
		return nil, notFound(err)
	}
	dto := batchWitnessToDTO(row)
	return &dto, nil
}

// MarkStatus delegates to the inner store, remapping a missing-row result
// to corehost.ErrNotFound.
func (a *WitnessQueueAdapter) MarkStatus(height, status int64) error {
	return notFound(a.inner.MarkStatus(height, status))
}

// --- proof store ---------------------------------------------------------

// ProofStoreAdapter adapts a *ProofStore to the corehost.ProofStore port.
type ProofStoreAdapter struct {
	inner *ProofStore
}

// NewProofStoreAdapter wraps the given *ProofStore as a
// corehost.ProofStore.
func NewProofStoreAdapter(inner *ProofStore) *ProofStoreAdapter {
	return &ProofStoreAdapter{inner: inner}
}

// proofToDTO projects a gorm Proof row onto the gorm-free port DTO. Every
// string payload is copied verbatim.
func proofToDTO(row *Proof) corehost.ProofDTO {
	return corehost.ProofDTO{
		ProofInfo:               row.ProofInfo,
		CexAssetListCommitments: row.CexAssetListCommitments,
		AccountTreeRoots:        row.AccountTreeRoots,
		BatchCommitment:         row.BatchCommitment,
		AssetsCount:             row.AssetsCount,
		BatchNumber:             row.BatchNumber,
	}
}

// proofFromDTO lifts a port DTO into a gorm Proof row (zero gorm.Model).
// Every string payload is copied verbatim.
func proofFromDTO(dto *corehost.ProofDTO) Proof {
	return Proof{
		ProofInfo:               dto.ProofInfo,
		CexAssetListCommitments: dto.CexAssetListCommitments,
		AccountTreeRoots:        dto.AccountTreeRoots,
		BatchCommitment:         dto.BatchCommitment,
		AssetsCount:             dto.AssetsCount,
		BatchNumber:             dto.BatchNumber,
	}
}

// EnsureSchema delegates to the inner store's CreateTable. Idempotent.
func (a *ProofStoreAdapter) EnsureSchema() error {
	return a.inner.CreateTable()
}

// Create maps the DTO to a gorm row verbatim and inserts it.
func (a *ProofStoreAdapter) Create(row *corehost.ProofDTO) error {
	r := proofFromDTO(row)
	return a.inner.Create(&r)
}

// GetByBatchNumber fetches the proof at the given batch height and projects
// it onto a DTO. A missing row surfaces as corehost.ErrNotFound.
func (a *ProofStoreAdapter) GetByBatchNumber(n int64) (*corehost.ProofDTO, error) {
	row, err := a.inner.GetByBatchNumber(n)
	if err != nil {
		return nil, notFound(err)
	}
	dto := proofToDTO(row)
	return &dto, nil
}

// ListAllInOrder returns every proof row, BatchNumber ascending, projected
// onto DTOs. An empty store yields an empty slice and a nil error.
func (a *ProofStoreAdapter) ListAllInOrder() ([]corehost.ProofDTO, error) {
	rows, err := a.inner.ListAllInOrder()
	if err != nil {
		return nil, err
	}
	out := make([]corehost.ProofDTO, len(rows))
	for i := range rows {
		out[i] = proofToDTO(&rows[i])
	}
	return out, nil
}

// --- user-proof store ----------------------------------------------------

// UserProofStoreAdapter adapts a *UserProofStore to the
// corehost.UserProofStore port.
type UserProofStoreAdapter struct {
	inner *UserProofStore
}

// NewUserProofStoreAdapter wraps the given *UserProofStore as a
// corehost.UserProofStore.
func NewUserProofStoreAdapter(inner *UserProofStore) *UserProofStoreAdapter {
	return &UserProofStoreAdapter{inner: inner}
}

// userProofToDTO projects a gorm UserProof row onto the gorm-free port DTO.
// Every string payload is copied verbatim.
func userProofToDTO(row *UserProof) corehost.UserProofDTO {
	return corehost.UserProofDTO{
		AccountIndex:    row.AccountIndex,
		AccountId:       row.AccountId,
		AccountLeafHash: row.AccountLeafHash,
		TotalEquity:     row.TotalEquity,
		TotalDebt:       row.TotalDebt,
		TotalCollateral: row.TotalCollateral,
		Assets:          row.Assets,
		Proof:           row.Proof,
		Config:          row.Config,
	}
}

// userProofFromDTO lifts a port DTO into a gorm UserProof row. Every string
// payload is copied verbatim.
func userProofFromDTO(dto corehost.UserProofDTO) UserProof {
	return UserProof{
		AccountIndex:    dto.AccountIndex,
		AccountId:       dto.AccountId,
		AccountLeafHash: dto.AccountLeafHash,
		TotalEquity:     dto.TotalEquity,
		TotalDebt:       dto.TotalDebt,
		TotalCollateral: dto.TotalCollateral,
		Assets:          dto.Assets,
		Proof:           dto.Proof,
		Config:          dto.Config,
	}
}

// EnsureSchema delegates to the inner store's CreateTable. Idempotent.
func (a *UserProofStoreAdapter) EnsureSchema() error {
	return a.inner.CreateTable()
}

// Create maps each DTO to a gorm row verbatim and inserts the batch.
func (a *UserProofStoreAdapter) Create(rows []corehost.UserProofDTO) error {
	out := make([]UserProof, len(rows))
	for i := range rows {
		out[i] = userProofFromDTO(rows[i])
	}
	return a.inner.Create(out)
}

// GetByIndex fetches the row at the given account index and projects it
// onto a DTO. A missing row surfaces as corehost.ErrNotFound.
func (a *UserProofStoreAdapter) GetByIndex(idx uint32) (*corehost.UserProofDTO, error) {
	row, err := a.inner.GetByIndex(idx)
	if err != nil {
		return nil, notFound(err)
	}
	dto := userProofToDTO(row)
	return &dto, nil
}

# Module + Profile Architecture

이 문서는 `ConstraintModule` 라이브러리와 Profile descriptor 의
architecture 결정 중 **지금 lock 되는 부분만** 담는다. impl 시점·
stage 순서는 `PRODUCTION_ROADMAP.md` 참조. 컨셉·V1 강한 보장·
scope 는 `01-project-context.md` 참조.

여기 박힌 것들은 추후 변경 시 versioned change 다 — 즉 새 `.vk`
ceremony / customer 마이그레이션 / 카탈로그 ID 변경이 동반된다.

## Source-of-truth 위치

| 우선순위 | 문서 | 이 문서와의 관계 |
|---:|---|---|
| 1 | `zkpor/core/spec/` 코드 (frozen 계약) | 인터페이스 시그니처가 우선. 이 문서는 그 인터페이스의 사용 규약. |
| 2 | `zkpor/docs/01-project-context.md` | 컨셉·strong guarantee. 이 문서는 그 컨셉의 module-layer 구체화. |
| 3 | **이 문서 (`02-module-architecture.md`)** | module + profile 의 architecture lock. |
| 4 | `zkpor/PRODUCTION_ROADMAP.md` | stage·timing. 이 문서의 lock 들은 ROADMAP 진행과 충돌해선 안 됨. |

# 1. ConstraintModule 의 본성 — Add-Only

`ConstraintModule.Define(api, ctx)` 는 base 회로 위에 **추가** 제약만
emit 한다. 기존 제약을 약화·제거할 수 없다.

이 한 가지 규칙이 다음을 자동으로 가져온다:

- **순서 무관**: 두 module 의 제약은 AND 결합 (commutative). 어느 순서로
  emit 해도 효과적 R1CS 시맨틱은 동일.
- **Composition 안전**: N module 의 제약 집합 union 은 그 자체로
  add-only. 새 module 1개 추가한 것과 수학적으로 같음.
- **부작용 격리**: module 끼리 데이터 흘려보낼 메커니즘 없음 (ctx 가
  read-only — 아래 §3).

이 자연 안전성이 §2 의 composition 규격을 가능하게 한다.

# 2. Composition 규격

## 2.1 Composition 의 의미

"Chaining" 이 아니라 "Composition". 두 module 이 같은 witness 에 각자
독립적으로 제약을 추가하는 것 — 데이터 파이프라이닝 아니다.

## 2.2 인터페이스는 단수 슬롯 유지

`BatchCreateUserCircuit.module ConstraintModule` 은 단수. 이 모양은
R3 step 2 (commit `ccc3fe4`) 에서 wiring 됐고 변경 없음. N 개 module
이 필요한 deployment 는 **Composite 패턴** 으로 처리.

## 2.3 Composite 패턴

```go
// core/constraint_modules/composite/composite.go  (impl 시점은 R5+)
type compositeModule struct { members []ConstraintModule }

func ComposeModules(members []ConstraintModule) ConstraintModule {
    return compositeModule{members: members}
}

func (c compositeModule) ID() ConstraintModuleID { /* §2.4 */ }
func (c compositeModule) Define(api API, ctx ConstraintContext) error {
    // §2.5 canonical sorted emit order
    for _, m := range sortByID(c.members) {
        if err := m.Define(api, ctx); err != nil { return err }
    }
    return nil
}
```

핵심 성질:

- composite 자체가 `ConstraintModule` — uniform 처리.
- N=1 일 때 `ComposeModules([m]).ID() == m.ID()` — backwards compat.
- profile 은 회로 instance 에 `SetConstraintModule(ComposeModules(...))`
  한 줄로 N module 을 합성.

## 2.4 Composite ID — Canonical Sort + Join

Composite 의 ID 는 **constituent ID 들의 lexicographic sort 결과를
`+` 로 join**. 단일 module 일 때는 그 module 의 ID 그대로.

```
{m1.ID(), m2.ID()}  →  sort  →  join("+")  →  composite.ID()

예:
  {"regulator.kr.user_limit_v1", "business.spot_only_v1"}
  →  "business.spot_only_v1+regulator.kr.user_limit_v1"
```

규칙:

- **sort 필수** — 입력 순서에 무관해야 같은 ceremony key 가 나옴.
- separator `+` 는 G9 의 ID 문자집합 (`lowercase, digits, dots,
  underscores`) 에 **추가됨** — G9 의 자연스러운 보강. 이 문서가 보강
  lock 으로 작용.
- 단일 module 일 때 = identity. composite 가 N=1 인 경우 ID 가 wrapper
  로 인해 달라지지 않음.

## 2.5 Emit 순서 — Canonical Sorted

`Define()` 안에서도 sorted 순서로 emit. 이유:

- R1CS bytes (variable allocation 순서 등) 까지 deterministic.
- ceremony 결과 (.pk/.vk) 가 input 순서에 dependent 하지 않음.
- G1 (byte-equivalence) 의 단일 key 보장.

# 3. ConstraintContext — Read-Only

`ConstraintContext` (t4_tiered_haircut_margin_3pool/spec/constraint.go) 는 module 에게
witness 의 read-only view 만 제공한다. 다음이 명시적으로 금지:

- module 이 ctx 의 필드를 수정 (없는 동작 — ctx 는 value struct).
- module 이 다른 module 에게 derived value 를 전달.
- module 이 base 회로의 intermediate variable 에 새 의미 부여.

각 module 은 자기가 필요한 derived value 를 **자기 안에서** 다시
계산한다 (in-circuit 비용 약간 증가하지만 composition 의 격리 유지).

만약 두 module 이 같은 derived value 를 자주 쓴다면 — 그건 그 derived
value 를 `core/circuit/` 헬퍼로 승격할 신호 (G11). 여기서는 모듈 간
ctx 공유로 풀지 않는다.

# 4. Param-As-Public-Input

module 이 parameter 를 갖는 경우 (e.g., `regulator.kr.user_limit_v1
{daily_limit = 1_000_000}`):

- **Params 는 public input 으로 들어와야 한다** — verifier 가 값을 보고
  확인한다.
- **In-circuit constant 로 다뤄선 안 된다** — 그러면 params 값마다
  ceremony 분기가 일어나 `.pk`/`.vk` 가 폭발한다.

결과:

- 한 (model, module, shape) ceremony 가 모든 params 값에 대해 재사용.
- verifier 가 published `.vk` + (proof, public input) 로 검증할 때
  params 값을 public input 으로 확인 → audit-friendly.
- 단점: params 가 너무 많거나 다양하면 public input vector 가 커짐 —
  module 당 params 개수를 design 단계에서 제어.

V1 의 `noopModule` 은 params 0 이라 이 규칙이 dormant. 첫 parameterized
module 도입 시점 (R5 candidate) 에 첫 적용.

# 5. Module Catalog Layout

**STATUS (R7)**: catalog v1 FROZEN at zero core entries. `core/constraint_modules/`
디렉터리 + governance 는 R7 에 lock. 첫 entry 는 R7+1 (customer signal
+ rule-of-three 충족).

```text
core/
└── constraint_modules/             ← V1 catalog (R7 frozen, 0 entries)
    ├── doc.go                      ← governance lock (R7-C)
    ├── regulator/                  (R7+1 candidate, prefix locked)
    │   └── <jurisdiction>/<rule>_v<v>/
    └── business/                   (R7+1 candidate, prefix locked)
        └── <pattern>_v<v>/

profile/<customer>/                 ← customer-specific module 거주처
    └── <module>.go                 id = "<exchange>.<rule>_v<v>"
```

## 5.1 ID Prefix 규약

| 위치 | Module ID 형식 | 예 |
|---|---|---|
| `core/constraint_modules/noop/` | `noop` | `noop` |
| `core/constraint_modules/regulator/` | `regulator.<jurisdiction>.<rule>_v<v>` | `regulator.kr.user_limit_v1` |
| `core/constraint_modules/business/` | `business.<pattern>_v<v>` | `business.spot_only_v1` |
| `profile/<customer>/` | `<exchange>.<rule>_v<v>` | `binance.vip_loan_carveout_v1` |

G9 가 closed 인 `<exchange>.<rule>_v<v>` 은 customer-specific 영역에
한정. core 라이브러리에는 위 표대로 더 명시적인 prefix 가 박힌다.

## 5.2 Rule-of-Three 와의 정합

`docs/01-project-context.md` 의 rule-of-three (패턴은 3번째 사례 후
코어 승격) 가 이 layout 의 자연스러운 동력:

```text
1st customer (Binance)         → profile/binance/constraint_noop.go
2nd customer (다른 거래소)      → profile/<X>/ 에 같은 noop 또는 새 module
3rd customer 같은 패턴 도입     → core/constraint_modules/ 로 승격
```

단 **`noop` 은 본질적으로 universal** 이지만 v1 시점에서 per-model
`ConstraintContext` field set 이 4 model 마다 다르므로 single generic
noop 타입이 비실용. 각 model 의 in-package noop helper
(`profile/<customer>/constraint_noop.go`) 가 현 시점 패턴. type-
parametric / interface-dispatched universal noop 은 v2 candidate.

**R7 freeze 결과** — `core/constraint_modules/` 디렉터리는 존재하되
entry 없음 (`doc.go` 만). 첫 promotion 은 R7+1 customer signal 시.

# 6. Profile Descriptor 진화 방향

V1 의 `profile/<customer>/` 은 Go 패키지 + 8 어댑터. 일부는 declarative
(asset list, batch shape, multipliers), 일부는 procedural (snapshot IO,
identity derivation, constraint module).

**진화 방향 (locked direction)**:

| Layer | 위치 | 진화 |
|---|---|---|
| **Declarative** (asset list, batch shape, multipliers, identity scheme ID, insolvent policy, source-type ID) | `profile/<x>/profile.toml` | R5-3 schema draft, **R7 schema v1 FROZEN** (`profile/declarative/profile.go`). |
| **Procedural — standard connectors** (csv_binance_v2 같은 표준 source) | `core/snapshot_connectors/` (R5+ 후보) | 두 번째 customer 가 같은 CSV 포맷 쓰면 promotion. |
| **Procedural — custom code** (customer-only module, custom snapshot, custom identity) | `profile/<x>/*.go` 그대로 | code 가 본질인 부분은 끝까지 code. registry 에 등록되어 ID 로 referenced. |

## 6.0 Profile descriptor schema v1 FROZEN (R7)

`profile/declarative/profile.go` 의 `Profile` struct + `Load` + `Validate`
가 v1 canonical schema. 같은 schema 가 4 model 모두 cover (per-model
required-field validation 은 `Validate()` 에서).

Reference instantiations:

- `profile/binance/binance.toml` — T4 model 사용 (3-bucket collateral)
- `profile/sea_reference/sea_reference.toml` — T1 model 사용 (spot debt=0)

v1 freeze 후 schema 변경 규약:

- 새 field (additive) — Load 에서 default 값 보장 → 기존 toml 파일 계속
  parse OK. minor schema bump.
- field 제거 — deprecate-then-remove 2 cycle minimum, deprecation
  window 에 parser warning.
- field rename — v1 에서 disallowed (기존 file 깨짐).
- 새 table — additive field 동일 규약.

**Service-startup wiring**: R7 시점에 schema 는 freeze 됐지만 service
startup 이 toml 을 직접 *consume* 하지는 않음 — procedural Go adapters
(`profile/<customer>/*.go`) 가 여전히 authoritative. R7+1 또는 V1
production deployment 시점에 wiring 전환 (각 adapter constructor 가
toml 값을 인자로 받는 refactor 동반).

**registry pattern** (R4+):
- ConstraintModule registry — `core/constraint_modules/` + 각 customer
  의 custom module 들이 ID 로 등록.
- Identity scheme registry — `passthrough_hex.v0`, `hmac_sha256.v1` …
- SnapshotSource registry — `csv_binance_v2`, `db_postgres_v1` …
- profile.toml 의 각 field 가 registry 의 ID 를 select.

이 진화의 핵심: **engine 빌드 시점에 registry 가 채워진다**. Go plugin
동적 로딩은 채택 안 함 (버전 깨짐 risk). 새 module / 새 connector 추가
= engine PR + 빌드.

## 6.1 Multi-customer `.vk` 공유 정책 (G12 closure)

두 customer (또는 더 많은) profile 이 같은 model 을 쓸 때 trusted-
setup artifact (`.pk`/`.vk`/`.r1cs`) 가 공유 가능한가 — R5 진입 시점에
명확히 답해야 했던 질문.

**답**: **공유 가능. r1cs 는 (model, asset_capacity, batch_shape,
constraint_module) tuple 으로 결정되며 customer profile 은 이 tuple
바깥의 값을 회로에 흘리지 않는다.**

근거:
- 회로 코드는 `core/solvency/<model>/circuit/*` — customer 패키지를
  import 하지 않음 (단방향 의존).
- `NewBatchCreateUserCircuit(userAssetCounts, allAssetCounts,
  batchCounts)` 시그니처에 customer 정보 없음.
- `ConstraintModule` 은 alpha-layer hook — module ID 가 같으면 emit
  되는 constraint 가 동일. `noop` 모듈은 두 customer 가 자유롭게 공유.
- `BatchShape.StandardKeyName` 도 이미 customer-blind:
  `zkpor.<model>.<tier>_<users>[.<module>]`. customer name 없음.

**운영 정책** (operator 책임 — R5 결정):

1. **`(model, asset_capacity, batch_shape, module)` tuple 이 같은
   customer 끼리 같은 `.vk` 디렉터리를 공유**. e.g. binance 와 SEA-
   customer 가 둘 다 `t4_tiered_haircut_margin_3pool`, capacity 500, shape `{50, 700}`,
   noop module 이면 같은 `zkpor.t4_tiered_haircut_margin_3pool.50_700.{pk,vk,r1cs}` 사용.

2. **`asset_capacity` 는 `StandardKeyName` 에 인코드되지 않으므로
   operator 가 명시적으로 일관성을 보장해야 한다**. `cmd/keygen` 의
   `-asset-capacity` 와 모든 service config 의 `AssetCapacity` 가
   같은 값을 가져야 함. 디렉터리 컨벤션 권장:
   `.artifacts/cap-<N>/zkpor.<model>.<tier>_<users>.*` — capacity
   별로 폴더 분리.

3. **profile name 은 `.vk` 신원의 일부가 아니다**. 두 profile 이
   같은 tuple 을 가지면 같은 ceremony 의 .vk 가 byte-equivalent 로
   동작. audit 라벨 측면에서 customer 별 파일명을 원하면 operator 가
   심볼릭 링크 또는 복사로 처리 — engine 은 stem 만 본다.

4. **다른 tuple 은 무조건 별도 ceremony**. shape 다름, capacity 다름,
   module 다름은 별도 `.vk`. profile 다른 건 무관.

**향후 (R7 freeze 직전 후보)**: capacity 를 StandardKeyName 에 인코드
(예: `zkpor.<model>.cap<N>.<tier>_<users>.<module>`) — operator 의
실수 위험을 줄이는 trade-off. R5 시점에는 컨벤션-only.

# 7. G16 — Composition Compatibility Process (방향 lock)

composition 자체는 §1 의 add-only 로 수학적으로 안전. 그러나
**module 간 hidden assumption 충돌** 은 발생 가능:

- 예: `business.spot_only_v1` 이 "system 에 debt 제약 없음" 을 전제.
- 다른 module 이 debt 와 collateral 관계를 assert.
- composition 시 unsat — proof 생성 실패 (audit 시 발견 안 될 수
  있음).

이 risk 의 mitigation 으로 G16 의 process direction lock:

1. **각 module 의 doc/audit note 에 assumed invariants 명시 의무**.
2. **composition 등록 (=새 `.vk` ceremony 시작) 전에 invariant
   compatibility 검토** — 형식은 review checklist, 자동화는 future
   work.
3. **첫 multi-module composition 등장 시 process detail 확정** —
   reviewer who, document where, fail-mode 등.

G16 content (process detail) 는 첫 multi-module customer 등장 시
PRODUCTION_ROADMAP 의 해당 stage 에서 채워진다. 이 문서는 process
**existence** 와 **방향** 만 lock.

# 8. 이 문서에서 lock 되지 않은 것

다음은 의도적으로 미정:

- **ConstraintContext 의 정확한 surface 확장** — V1 에는 minimal,
  첫 non-noop module 등장 시 결정 (G3).
- **multi-tenant `.vk` 공유 정책** — G12, R4 진입 시.
- **profile.toml schema 의 정확한 format** (TOML vs YAML vs JSON),
  필드 이름, 버전 표기 — R4 에서 emerging, R7 freeze.
- **composition 의 ceremony 비용 amortization 전략** — 후보가 많은
  module set 에서 어느 조합을 미리 ceremony 해둘지 — 운영 시점 결정.
- **G16 process 의 구체 절차** — 첫 multi-module composition 시.

이들은 명시적으로 후속 결정 항목이며, 이 문서가 mature 해질 때 (V1
중반 이후) 추가될 수 있다.

# 9. 이 문서 변경 시

- 추가 (새 lock 박기): 신규 PR + 관련 stage 의 `Blocking gates` 갱신.
- 변경 (기존 lock 수정): versioned change. 영향 받는 ceremony / customer /
  `.vk` 의 마이그레이션 plan 동반.
- 삭제 (lock 풀기): 명시적 가능. 단 그 lock 에 의존하는 코드/문서 검토
  의무.

이 문서의 source-of-truth priority 는 `01-project-context.md` 와
`PRODUCTION_ROADMAP.md` 사이 (priority 3 — ROADMAP Source-of-truth 표
참조).

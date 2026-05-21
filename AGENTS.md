# AGENTS.md

이 파일은 `zkpor/` 엔진에 대한 agent contract이다. 세션 cwd는 project root
(`zkmerkle-proof-of-solvency/`) 기준. 작업 시작 시 가장 먼저 읽는다.

## Project Context

`zkpor/` 는 sibling `circuit/`, `src/`(Binance OSS zk-PoR v2)를 다거래소
N-customer **PoR 엔진 제품**으로 생산화하는 R&D 작업 공간이다. SaaS 형태로
엔진 프로그램을 판매하는 것이 목표이며, 호스팅 인프라·verifier 웹 UI·
컴플라이언스 인증은 범위에 포함되지 않는다.

초기 제품 범위 전제:

- **V1 scope**: `tier_3bucket` 모델 (Binance reference 이식) + `binance`
  프로파일 end-to-end 동작.
- **확장 방향**: `spot_simple`, `merkle_classic`, `over_collateral_simple`,
  `tier_1bucket` — 카탈로그 등재만 유지, 회로 구현은 고객 신호 따라.
- **목표 고객 / 운영 / SLA**: 마진·론 사업 거래소(Binance/OKX-class) 우선 +
  한국·EU 규제 spot 거래소 / managed SaaS 엔진 통합 / 고객 onboarding 1~4개월.

강하게 말할 수 있는 보장: **"주어진 user balance dataset이 published CEX
totals와 산술적으로 일치하며, 선택된 솔밴시 모델의 각 사용자 조건을 통과한다."**

zk 단독으로 보장하지 않는 것: **거래소가 dataset에 포함시키지 않은 사용자/
계정의 존재**. 보완책: 사용자 self-inclusion verifier + AccountIDProvider
scheme 공개 + 외부 audit.

## Repository Direction

- `circuit/`, `src/` — legacy Binance OSS PoR v2 reference. **수정
  금지**. 현재 동작하며 trusted setup 그대로 유효.
- `zkpor/` — 신규 modular engine. 자체 git 저장소 (`zkpor/.git/`).
  모든 신규 작업은 여기.
  - `zkpor/core/spec/` — universal interfaces + 5-tier catalog 상수.
  - `zkpor/core/circuit/` — universal zk helpers (Merkle, commitment, arith).
  - `zkpor/core/solvency/<model>/` — audited math 카탈로그. 5-tier.
  - `zkpor/profile/<customer>/` — 고객사 deployment.
  - `zkpor/docs/` — methodology 문서 (project-context).
  - `zkpor/PRODUCTION_ROADMAP.md` — Part 3 (stages + gates).
  - `zkpor/HANDOFF.md` — 현재 시점 인수인계.
  - `zkpor/CLAUDE.md` — Claude 자동 로드 메모리 (AGENTS.md redirect).
- `docs/` — historical design exploration. source 아님.

핵심 경계 — **Solvency Model (math)** vs **Profile (deployment)** 은 직교
축이다. 같은 model을 여러 profile이 공유할 수 있다. v1은 한 model당 한
회로 instance를 만든다 (composition 미지원).

## Implementation Principles

- **Demo shortcut보다 production 계약 경계를 우선한다.** statement/encoding/
  witness 경계를 임시로 바꾸지 않는다.
- **proof backend(또는 핵심 기술)를 먼저 고정하지 않는다.** canonical type,
  encoding, public statement, account-leaf schema를 먼저 고정한다. 본
  엔진은 이미 Poseidon over BN254 + SMT depth 28을 고정했다.
- **입력 정렬·중복·endian·range·omitted field 정책은 코드와 fixture test로
  고정한다.** legacy `src/utils/utils.go`의 ETL 동작은 reference이며, 신규
  adapter는 이를 보존한다.
- **raw 입력은 그대로 핵심 경계에 넣지 않는다.** adapter/normalizer가
  canonical 형태로 변환한다. snapshot CSV → AccountInfo는 `zkpor/profile/
  <customer>/snapshot.go` 책임.
- **slice = commit.** 변경은 작게 나누고, docs/scaffold → implementation →
  tests 순서로 분리한다. refactor·feature는 별도 커밋.
- **미결정·spec 공백·frozen 계약 불일치는 debate/question으로 surface한다.**
  agent가 임의로 결정하지 않는다.
- **검증 명령을 실제로 실행하기 전에는 완료를 선언하지 않는다.**

추가로 본 엔진에 고유한 규칙:

- `circuit/`, `src/` legacy 코드는 직접 수정 금지. 변경이 필요하면
  `zkpor/` 안에 신규 코드로 추가하고 점진적으로 대체한다.
- `zkpor/core/solvency/` 추가(=새 모델 카탈로그 등재)는 audit + trusted setup
  ceremony 가 필요한 변경이다. 단순 PR로 처리 금지, 별도 거버넌스로 처리.
- `ConstraintModule`은 add only — 기존 base-circuit 제약을 weaken/remove
  하지 않는다. trusted setup 분기됨.
- 고객사 이름을 model identifier에 박지 않는다. `tier_3bucket`, not
  `binance_v2`. 고객사 이름은 `zkpor/profile/<customer>/` 디렉터리 이름에만.
- `PriceMultiplier × BalanceMultiplier == ValueScale` 불변식. 기본 1e16
  (1e8 × 1e8). 서비스가 startup에서 assert 한다.

## Go Documentation Rule

공개 API에 godoc을 단다. 적용 대상: exported identifier (대문자 시작 타입/
함수/메서드/필드).

문서에는 구현 설명보다 다음을 우선 기록한다.

- 이 타입/함수가 PoR 엔진에서 맡는 역할.
- deterministic/canonical 처리와 관련된 불변조건.
- 정렬/중복/범위/overflow 정책.
- 핵심 경계(zk 회로, public statement, account leaf hash, batch commitment)
  와 연결되는 의미.
- validation 함수의 실패 의미.

## Testing And Harness Rules

- 신규 구현은 다음 baseline 명령으로 검증한다 (project root에서 실행).

  ```bash
  go build ./zkpor/...
  go vet ./zkpor/...
  go build ./...           # legacy + 신규 (legacy 영향 없음 확인)
  ```

- 회로 코드를 이식하면 trusted setup byte-equivalence를 검증한다 (`.pk`/
  `.vk` 파일 hash 비교).
- fixture는 happy 케이스와 tamper(실패해야 하는) 케이스를 함께 둔다.
- hash/root/canonical bytes는 expected fixture로 freeze한다.
- 성능 측정은 correctness baseline이 안정된 뒤 별도 benchmark에서 다룬다.

## Communication Notes

- 프로젝트 문서와 논의 기록은 기본 **한국어**.
- 코드 identifier, schema field, CLI command, error code는 **영어**.
- 설계 판단은 **"왜 이 구조가 customer integration cost + audit trust로
  이어지는지"** 를 기준으로 설명한다 (제품화·SaaS 관점).
- 새 모델 / 새 customer / 새 ConstraintModule 등 카탈로그·계약 영향이 있는
  결정은 단발 결정으로 두지 말고 `zkpor/PRODUCTION_ROADMAP.md`의 gate
  register에 반영한다.

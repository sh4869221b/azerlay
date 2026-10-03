# Azerlay システム設計書

**文書版:** 1.0  
**作成日:** 2026-09-01  
**対象リリース:** Azerlay 1.0  
**状態:** 実装開始前の基準設計  
**実装言語:** Go  
**対象OS:** Linux / Wayland

---

## 0. 設計要約

| 項目 | 決定 |
|---|---|
| 製品名 | **Azerlay** |
| GitHubリポジトリ | **`azerlay`** |
| 製品形態 | Linux/Wayland専用、単一プロセス・単一バイナリのAzeron入力オーバーレイ |
| 初期正式対応機器 | Azeron Cyborg II 左手用（v1）。右手用はv1後の対応へ延期 |
| 初期正式対応Compositor | Hyprland（v1目標）。Niriはv1後へ延期し、動作未検証 |
| 初期ゲームプロファイル | Bodycam |
| 実装言語 | Go 1.27.x |
| GUI | GTK4 / gotk4 |
| Overlay | gtk4-layer-shellを小型CGo bridgeから利用 |
| 入力 | 限定interface04 hidraw、受動read-only、非grab、非inject |
| 描画 | GTK4 DrawingArea + Cairo |
| プロファイル入力 | Azeron exportの厳密import、およびAzeron Softwareローカル設定のread-only取得 |
| 確認済みexport pipeline | Base64URL → LZMA-Alone → MessagePack String → UTF-8 JSON |
| 設定 | TOML、XDG Base Directory準拠 |
| 実行時制御 | CLI + 0600 Unix Domain Socket |
| 権限 | 対象Azeron限定udev `uaccess`。root・`input`グループ不要 |
| ネットワーク | 使用しない。テレメトリーなし |
| 公開ライセンス | MPL-2.0推奨。公開前に最終確認 |

設計上の最大の不確定要素は、Azeron Software 2.xの完全なexport型対応表、Cyborg IIのinput IDと物理ボタン位置の対応、Linux版Azeron Softwareローカル保存形式、機器改版ごとのVID/PIDとHID report構成、gotk4とgtk4-layer-shellの実機ABI連携である。これらは本書の「要調査事項」でリリース阻害範囲を明記する。

---

## 1. 文書の目的

本書は、Azeron Cyborg II のキーバインドとリアルタイム入力状態を、Linux/Wayland上でゲーム画面へ重ねて表示するアプリケーション **Azerlay** の完全な製品要件、アーキテクチャ、データ設計、運用要件、検証条件を定義する。

本書はPoC向けの暫定設計ではない。Azeronプロファイルのインポート、Azeron Softwareローカル設定の自動読取、入力デバイス監視、Waylandオーバーレイ、設定管理、障害復旧、配布、セキュリティ、テスト、互換性管理までを製品要件として扱う。

実装時のレビュー単位、コミット分割、Pull Request分割、作業順序は本書では定義しない。これらは実装開始時に、本書の境界と受入条件を基に別途決定する。

### 1.1 規範語

本書では以下を用いる。

- **MUST:** 製品成立に必須。
- **SHOULD:** 原則として実装する。除外する場合は理由を記録する。
- **MAY:** 任意機能。
- **要調査:** 実装前またはリリース前に実機・現行ソフトウェア・現行APIで確認が必要。
- **リリース阻害:** 未解決のまま対象機能を正式サポートとして公開してはならない。

---

## 2. プロジェクト定義

### 2.1 正式名称

| 項目 | 値 |
|---|---|
| プロジェクト名 | **Azerlay** |
| 読み | アゼルレイ |
| GitHubリポジトリ名 | **`azerlay`** |
| 実行バイナリ | **`azerlay`** |
| Go module | `github.com/<owner>/azerlay` |
| Desktop Application ID | `io.github.<owner>.azerlay` |
| Wayland Layer namespace | `azerlay` |
| systemdユーザーサービス | `azerlay.service` |
| 設定ディレクトリ | `$XDG_CONFIG_HOME/azerlay` |
| データディレクトリ | `$XDG_DATA_HOME/azerlay` |
| ランタイムディレクトリ | `$XDG_RUNTIME_DIR/azerlay` |

GitHubリポジトリ作成時には、`azerlay`という名称が利用可能か再確認する。2026-09-01時点の検索では同名の公開リポジトリは確認されていないが、名前の確保を保証するものではない。

### 2.2 名前の意図

**Azeron + Overlay** を短縮した名称であり、次を満たす。

- 実行コマンドとして短い。
- 特定ゲーム名を含まず、汎用ツールへ拡張できる。
- 「入力表示」「キーバインド表示」「Waylandオーバーレイ」という製品目的を説明しやすい。

### 2.3 GitHubリポジトリ説明文

```text
Wayland-native Azeron profile and live input overlay for Linux.
```

日本語説明:

```text
Azeronプロファイルとリアルタイム入力を表示するLinux/Waylandネイティブオーバーレイ。
```

### 2.4 ライセンス

推奨ライセンスは **MPL-2.0** とする。

理由:

- オープンソースとして利用・改変・再配布しやすい。
- ファイル単位のコピーレフトであり、派生物の改善を還元させつつ利用側への制約を過度に増やさない。
- GTK、gotk4、gtk4-layer-shell等の依存関係と組み合わせやすい。

公開前に依存ライセンス一覧、NOTICE、SBOMを生成し、ライセンス互換性を再確認する。これは公開リリース前の必須確認である。

### 2.5 商標・非公式表示

README、About、配布ページに次の趣旨を明記する。

```text
Azerlay is an independent, unofficial project and is not affiliated with,
endorsed by, or sponsored by Azeron SIA. Azeron and Cyborg are trademarks
of their respective owner.
```

公式製品画像、公式UI素材、ロゴを無断で同梱しない。Cyborg IIの表示図は、物理配置を示す独自の模式図として作成する。

---

## 3. 製品概要

Azerlayは、Azeron Softwareからエクスポートしたプロファイル、またはローカルに保存されたAzeron Software設定を読み取り、各物理ボタンの割当をCyborg IIの模式図上へ表示する。確認済みinterface04 hidrawのtype57通知から30物理ボタンの最終観測状態を得る。analog・live出力キー・trigger発火完了は未対応で、入力からrenderer/runへの接続は別作業とする。

オーバーレイはWayland Layer Shellの`overlay`層に配置し、キーボードフォーカスを取得せず、ポインター入力を透過する。ゲーム入力を横取り、再送、変換しない。

Bodycamは最初に同梱するゲームラベルプロファイルと検証対象である。ただしAzerlay本体はゲーム非依存とする。

### 3.1 代表的な利用状態

```text
Azeron Cyborg II
      │
      ├── Azeron profile export / local profile store
      │                       │
      │                       ▼
      │                keybind definitions
      │
      └── /dev/hidrawN
                              │
                              ▼
                         live input state
                              │
                              ▼
                        Azerlay reducer
                              │
                              ▼
                  GTK4 + Cairo overlay model
                              │
                              ▼
                    gtk4-layer-shell overlay
                              │
                              ▼
                   Bodycam / other Wayland game
```

---

## 4. 目標と非目標

### 4.1 目標

1. Azeronの既存設定を二重入力せず、プロファイルから物理ボタンと割当を復元する。
2. ゲーム中に確認しやすいCyborg II模式図を、低遅延・低負荷で表示する。
3. キー、マウスボタン、ゲームパッドボタン、アナログスティックをリアルタイム表示する。
4. Azeron固有のラベル、長押し、ダブルタップ、マクロ、ターボ、アナログ設定を欠落させず内部表現へ正規化する。
5. v1ではHyprlandで、フルスクリーンゲーム上にクリック透過表示する。Niriはv1後へ延期し、動作未検証とする。
6. root実行、`input`グループへの恒久追加、入力デバイスの排他的取得を必要としない。
7. 不正・破損・未知バージョンのプロファイルを安全に拒否し、最後に正常だった設定で動作を継続する。
8. オフラインで動作し、テレメトリーや外部通信を行わない。
9. 単一バイナリ、単一プロセスを基本とし、不要なデーモン分割やWeb UIを導入しない。

### 4.2 非目標

以下はAzerlay 1.0の対象外とする。

- Azeronデバイスへの設定書き込み。
- ファームウェア更新、キャリブレーション、LED制御。
- キーリマップ、入力注入、マクロ実行。
- `EVIOCGRAB`による入力独占。
- WindowsまたはmacOS対応。
- X11対応。
- GNOME WaylandでのLayer Shell非対応環境の代替実装。
- OBS映像への焼き込みを目的としたプラグイン。
- ゲームごとの自動キー設定変更。
- Azeron Softwareの代替設定アプリ。
- クラウド同期、アカウント、ネットワークサービス。
- WebView、Electron、Tauri、React等によるUI。
- 公式Azeron外観を複製したフォトリアル表示。

### 4.3 将来拡張として許容するもの

設計上は拡張可能にするが、1.0の正式サポートとはしない。

- Azeron Classic、Compact、Cyborg、Cyborg II Compact等の追加モデル。
- `hidraw`によるオンボードプロファイルのread-only取得。
- ゲーム実行プロセスに応じたプロファイル自動切替。
- 設定GUI、トレイアイコン。
- OBS向け透過出力。
- 多言語ゲームプロファイル配布。

---

## 5. 対応環境

### 5.1 正式サポート

| 項目 | 要件 |
|---|---|
| OS | Linux（v1はArch/CachyOS。Ubuntuはv1後） |
| Display server | Wayland |
| Compositor | Hyprland（v1）。Niriはv1後、動作未検証 |
| 入力API | Linux hidraw（確認済み受動reportのみ） |
| 対象機器 | Azeron Cyborg II 左手用（v1）。右手用はv1後の対応へ延期 |
| CPU architecture | x86_64 |
| UI toolkit | GTK4 |
| Layer protocol | `zwlr_layer_shell_v1` |
| Go | Go 1.27.xを基準に固定 |

### 5.2 準対応

Layer Shellを実装する以下の環境は、動作確認後に「準対応」として記載できる。

- Sway / wlroots系Compositor
- KDE Plasma Wayland
- SmithayベースCompositor
- GamescopeセッションまたはネストされたGamescope

### 5.3 非対応

- GNOME Shell Wayland: Layer Shellを標準サポートしないため正式非対応。
- X11/XWayland専用セッション: オーバーレイ実装対象外。
- Compositorを介さないDRM/KMS直接描画ゲーム。
- Ubuntuはv1動作対象外。[Issue #98](https://github.com/sh4869221b/azerlay/issues/98)でv1後の実装・検証を扱う。

### 5.4 ターゲット実機

初期の必須検証環境は次とする。

- CachyOS / Arch Linux系
- NVIDIA proprietary driver
- Hyprland
- 複数モニター
- 高リフレッシュレート、VRR
- Proton上のBodycam

HDR、VRR、Direct Scanoutとの相互作用は要調査であり、正式サポート表に結果を記載する。
Niriの実機検証はv1の必須マトリクスに含めず、v1後に行う。

---

## 6. ユースケース

### UC-001 エクスポートプロファイルのインポート

利用者はAzeron Softwareでエクスポートした文字列またはファイルをAzerlayへ渡す。Azerlayは形式を自動検出し、含まれるプロファイル一覧を表示して1つを選択、永続化する。

### UC-002 クリップボード由来文字列のインポート

利用者はエクスポート文字列を標準入力へ渡す。

```bash
wl-paste | azerlay import -
```

Azerlay自身はクリップボードを常時監視しない。

### UC-003 ローカルAzeron設定の自動読取

AzerlayはAzeron Softwareのローカルプロファイル保存場所を検出し、選択プロファイルを読込む。ファイル変更時は自動再読込する。

この機能の保存場所、形式、排他制御は公式外部APIとして確定していないため、**要調査・必須機能**とする。読取はread-onlyで行い、Azeron Softwareのファイルを変更しない。

### UC-004 オーバーレイ開始

利用者が`azerlay run`を実行すると、選択済みプロファイルと設定を読み込み、指定モニター・位置へオーバーレイを表示する。

### UC-005 ゲーム中の入力表示

Azeronのボタンを押すと、対応する模式図のコントロールをハイライトする。親指スティックは方向と入力量を表示する。

### UC-006 プロファイル切替

利用者はCLIまたは制御ソケット経由でプロファイルを選択する。動作中の場合、完全に検証済みの新スナップショットへ原子的に切替える。

### UC-007 設定破損時の継続

設定、ゲームプロファイル、Azeronプロファイルの再読込に失敗した場合、Azerlayは現在の正常なスナップショットを保持する。エラーを通知するが、オーバーレイを消去しない。

### UC-008 デバイス抜差し

Azeronを外すと「Disconnected」を表示し、読取goroutineを終了する。再接続後は同じシリアルまたはデバイス識別情報を用いて自動復帰する。

### UC-009 診断

`azerlay doctor`はWayland、Layer Shell、GTK、権限、デバイス、プロファイル、モニター、設定、制御ソケットを検査し、具体的な修正方法を表示する。

---

## 7. 機能要件

### 7.1 プロファイル入力

| ID | 要件 |
|---|---|
| FR-001 | Azeronエクスポートをファイル、標準入力、引数文字列からインポートできること。 |
| FR-002 | 前後の空白、改行、Markdownコードフェンス、単一・二重引用符の包みを安全に除去できること。 |
| FR-003 | Raw JSON、Base64URL、標準Base64、LZMA-Alone圧縮ペイロードを判別すること。誤判定時は候補を順に試すのではなく、明確な形式検査を行うこと。 |
| FR-004 | 提供サンプルで確認した `Base64URL → LZMA-Alone → MessagePack string → UTF-8 JSON` を正式に処理すること。 |
| FR-005 | MessagePackの`fixstr`、`str8`、`str16`、`str32`をサポートし、宣言長と残バイト数の完全一致を検証すること。 |
| FR-006 | JSONルートが`version + profiles`のバンドル形式、または単一プロファイル形式の双方を処理すること。 |
| FR-007 | 破損ストリームから部分的に復元できたJSONを採用してはならないこと。全段階が成功した場合のみ保存すること。 |
| FR-008 | 入力データのSHA-256を算出し、同一ソースの重複インポートを識別すること。 |
| FR-009 | 元データを必要最小限の権限で保存し、由来、取込日時、デコーダ版、エクスポート版を記録すること。 |
| FR-010 | 不明フィールドを理由に全体を拒否せず、既知必須フィールドを検証し、不明フィールドは将来互換情報として保持または診断表示すること。 |
| FR-011 | 未知の形式・版を推測で解釈せず、`unsupported format/version`として明示的に停止すること。 |

### 7.2 Azeronデータの正規化

| ID | 要件 |
|---|---|
| FR-020 | 物理入力ID、pin情報、通常・長押し・ダブルタップ割当、ラベル、マクロ、ターボ、アナログ設定を内部モデルへ正規化すること。 |
| FR-021 | 旧形式の数値文字列、JSON数値、現行形式の`KeyW`、`AltLeft`等の記号形式を同一のCanonical Bindingへ変換すること。 |
| FR-022 | キーボード、マウス、ゲームパッド、アナログ軸、マクロ、未割当、不明割当を区別すること。 |
| FR-023 | 正規化不能な値を捨てず、`UnknownBinding`として生値と出典を保持すること。 |
| FR-024 | Azeron Softwareの形式世代ごとにVersioned Adapterを用意し、UIや入力処理へRaw JSON形式を漏らさないこと。 |
| FR-025 | プロファイルに含まれる空ラベルと非空ラベルを区別すること。 |
| FR-026 | 同じキーへ複数の物理入力が割り当てられている状態を保持すること。 |
| FR-027 | 左右スティック、デッドゾーン、角度、反転、回転、アナログ／WASD出力を欠落させないこと。 |

現行のv1実装はSoftware 2.0.2の[閉じた変換行](decisions/binding-conversion.md)に限る。T Turboの25/10回毎秒、W 50 ms→Delay 100 msのMacro設定とrepeat、Xbox角度0/90を型付きで保持する。Turbo/Macroは実行せず、90度の座標投影も行わない。legacy numeric変換はv1対象外で、v1後の[Issue #102](https://github.com/sh4869221b/azerlay/issues/102)へ延期する。上表の広い機能要件を現行adapterの対応範囲と読み替えない。

### 7.3 プロファイルソース

| ID | 要件 |
|---|---|
| FR-030 | `ImportedSource`を必須実装とすること。 |
| FR-031 | `AzeronLocalSource`を製品要件とし、ローカルAzeron Software設定をread-onlyで取得すること。保存場所と形式は要調査。 |
| FR-032 | ローカルソースの変更を監視し、安定化待ち後に再読込すること。 |
| FR-033 | ファイル書換途中を読まないよう、変更通知後のdebounceと再試行を行うこと。 |
| FR-034 | ローカルソースが見つからない場合、インポート済みソースへ自動フォールバックできること。 |
| FR-035 | ソースの優先順位、選択プロファイル、最後に正常だったスナップショットを永続化すること。 |
| FR-036 | Azeron Softwareが起動中でも設定ファイルを破壊・ロック・変更しないこと。 |
| FR-037 | オンボード設定の直接HID読取は1.0必須要件としない。将来追加する場合もread-onlyとすること。 |

### 7.4 デバイス検出と入力読取

| ID | 要件 |
|---|---|
| FR-040 | sysfs class/hidrawとUSB ancestryからVID/PID/release、interface04、確認済みreport descriptorを照合すること。名前だけで同定しない。 |
| FR-041 | 同一USB device親でグループ化し、一意にadmittedなinterface04だけをcompleteとすること。 |
| FR-042 | 選択したAzeronのinterface04 hidrawだけを読み、devices/doctor/reconnectを含めevdevをopenしないこと。 |
| FR-043 | 確認済み64-byte vendor type57、宣言長2のsource/stateだけを解釈し、他typeとpaddingを解釈しないこと。 |
| FR-044 | 確認済みsourceのstate1/0を最終観測press/releaseとして扱い、repeatを推測しないこと。 |
| FR-045 | 未検証のanalog、live出力キー、trigger発火完了を生成しないこと。静的設定は保持する。 |
| FR-046 | 受信した有効な物理reportごとに不変snapshotを確定すること。 |
| FR-047 | 初期・disconnect・reopen・検出したloss・不正reportはunknownとし、個別の有効reportだけで観測を回復すること。末尾欠落未検出によるstale状態を制約として明示する。 |
| FR-048 | 選択済みUSB identityを維持してhidraw番号変更・権限復帰を再試行すること。公式Softwareの通知復旧は別途実機確認する。 |
| FR-049 | 入力デバイスをgrabしないこと。 |
| FR-050 | 入力を注入・再送・加工しないこと。 |
| FR-051 | root権限で実行しないこと。 |

### 7.5 物理コントロール同定

| ID | 要件 |
|---|---|
| FR-060 | Cyborg IIのプロファイル`input ID`を模式図上のPhysical Controlへ対応付けること。左右モデルを区別すること。 |
| FR-061 | 確認済みtype57 source IDからPhysical Controlを求め、出力候補を物理元の推定に使わないこと。 |
| FR-062 | 重複出力でも通知された物理controlだけをknownにし、通知のないcontrolを推測しないこと。 |
| FR-063 | 静的な出力候補の重複・Unknown・Unbound情報を保持すること。 |
| FR-064 | 長押し・ダブル・マクロは静的割当表示のみとし、物理pressから発火・完了や実出力を推論しないこと。 |
| FR-065 | 確認済み30 source IDのhidraw受動通知を使用する。evdev fallback、HID writes/Feature/Output/GET_INPUT要求を行わない。 |

[#108](https://github.com/sh4869221b/azerlay/issues/108) の導入後に再評価し、現行v1では
物理sourceをtype57通知から直接同定できるため、出力列を照合するsequence matcherは不要と判断した
([#46](https://github.com/sh4869221b/azerlay/issues/46))。FR-064のLong/Double/Macroは静的割当表示のみで、
press/releaseやcounter、snapshot Sequenceは発火・完了・出力進行の証拠にならない。matcherは実装せず、
発火・進行表示は明示的な製品要件と別途根拠のある信号がある場合に限る。物理状態は初期unknownで、
検出できない末尾release欠落ではstaleになり得る最終観測状態であり、runtime/GTKにも未接続である
([構成境界](architecture.md#analog-and-output-state))。

### 7.6 オーバーレイ表示

| ID | 要件 |
|---|---|
| FR-070 | GTK4ウィンドウをLayer Shellの`overlay`層として生成すること。 |
| FR-071 | Layer namespaceを`azerlay`とすること。 |
| FR-072 | キーボードフォーカスを要求しないこと。 |
| FR-073 | GDK Surfaceの空Input Regionを用い、ポインター入力を完全透過すること。 |
| FR-074 | 画面の排他的領域を確保せず、ゲームのワークエリアを変更しないこと。v1のHyprland配置ではexclusive zone `-1`を用いる（[実測と限界](decisions/compositor-placement.md)）。 |
| FR-075 | v1ではモニターをconnector名の完全一致、次に一意な説明の完全一致で選択すること。明示指定が不在・曖昧なら非表示で状態を保持し、他出力へ移動しない。永続的な物理IDは保証しない。 |
| FR-076 | 左上・上・右上・左・中央・右・左下・下・右下のanchorと、X/Y marginを設定できること。 |
| FR-077 | scale、opacity、font scale、背景表示、プロファイル名表示を設定できること。 |
| FR-078 | 高DPIおよびfractional scaling環境で物理サイズ・文字が破綻しないこと。 |
| FR-079 | v1ではCyborg II左手用の独自模式図を表示すること。右手用はv1後の対応へ延期する。 |
| FR-080 | 各Physical Controlにアクションラベル、実際の割当、トリガー種別を表示できること。 |
| FR-081 | 押下中、解放遷移、長押し成立、ダブルタップ、マクロ実行候補、未割当、未知割当、曖昧性を視覚的に区別すること。 |
| FR-082 | アナログスティックを中心点、ベクトル、入力量、デッドゾーンで表示すること。WASDモードでは方向セグメントを表示すること。 |
| FR-083 | 描画領域不足時は文字の省略、縮小、優先度制御を行い、重なったまま放置しないこと。 |
| FR-084 | 日本語、英語、Unicodeラベルを表示できること。 |
| FR-085 | デバイス切断、プロファイル不在、再読込失敗をゲームを妨げない小さな状態表示で通知できること。 |
| FR-086 | オーバーレイ表示・非表示を実行中に切替できること。 |

### 7.7 ラベル解決

表示ラベルの優先順位は次とする。

1. AzerlayのPhysical Control単位ユーザー上書き。
2. Azeronプロファイルの非空`label`。
3. ゲームプロファイルのCanonical Binding→Action対応。
4. Canonical Bindingの人間向け表記。
5. 不明値の場合は`Unknown (<raw>)`。

実際の割当名はラベルとは別に副表示できなければならない。例えば`Lean Left`の下に`Q`を表示する。

| ID | 要件 |
|---|---|
| FR-090 | ラベル解決を上記優先順位で決定論的に行うこと。 |
| FR-091 | Azeronラベルが空のときだけゲームラベルへフォールバックすること。 |
| FR-092 | ゲームプロファイルが存在しなくてもキー名のみで利用可能なこと。 |
| FR-093 | ユーザー上書きは物理入力IDとトリガー種別をキーにし、同一キーを使う別ボタンへ誤適用しないこと。 |

### 7.8 設定・運用制御

| ID | 要件 |
|---|---|
| FR-100 | XDG Base Directory仕様に従うこと。 |
| FR-101 | TOML設定を読み、schema versionを検証すること。 |
| FR-102 | 設定変更を監視し、完全な候補設定を検証した後に原子的に反映すること。 |
| FR-103 | 制御用Unix Domain Socketを`$XDG_RUNTIME_DIR/azerlay/control.sock`へ作成し、0600とすること。 |
| FR-104 | CLIからshow、hide、toggle、reload、status、profile select、quitを実行できること。 |
| FR-105 | 同一ユーザーで複数インスタンスを誤起動しないこと。既存socketの生存確認とstale socket処理を行うこと。 |
| FR-106 | `systemd --user`サービスまたはCompositorの自動起動から実行できること。 |
| FR-107 | 正常終了時にsocket、goroutine、event fdを確実に閉じること。 |

### 7.9 診断

| ID | 要件 |
|---|---|
| FR-110 | `azerlay doctor`を提供すること。 |
| FR-111 | Waylandセッション、Layer Shell対応、GTK/共有ライブラリ、モニター、Azeron検出、hidraw権限、udev rule、プロファイル形式、設定、socket状態を検査すること。 |
| FR-112 | エラーコード、短い説明、対象パス、推奨修正コマンドを表示すること。 |
| FR-113 | `--json`で機械可読な診断結果を出力できること。 |
| FR-114 | 生のキーバインドや個人用ラベルを診断ログへ出す場合は明示オプションを必要とすること。 |

---

## 8. Azeronエクスポート形式

### 8.1 提供サンプルから確定した構造

提供された正常サンプルは、次のパイプラインで復元できる。

```text
Base64URL文字列
    ↓
Base64URL decode
    ↓
LZMA-Alone stream
    ↓
MessagePack String object
    ↓
UTF-8 JSON
    ↓
Azeron export bundle
```

正常サンプルの展開後先頭は`DA D5 D3`であった。

- `0xDA`: MessagePack `str16`
- `0xD5D3`: 文字列長54,739 bytes
- MessagePackヘッダー3 bytes + JSON 54,739 bytes = 展開サイズ54,742 bytes

もう1つの提供サンプルは途中で破損していたが、展開済み先頭は`DB 00 01 37 A7`であった。

- `0xDB`: MessagePack `str32`
- `0x000137A7`: 宣言文字列長79,783 bytes
- ヘッダー5 bytes + JSON 79,783 bytes = LZMAヘッダーの展開予定サイズ79,788 bytes

この結果から、以前の「任意prefix後の最初の`{`を検索する」という方式は採用しない。MessagePack envelopeを厳密に解釈しなければならない。

### 8.2 デコード手順

```text
1. Input normalization
2. Outer encoding detection
3. Base64 decode
4. LZMA-Alone header validation
5. Bounded LZMA decompression
6. MessagePack string header decode
7. Exact payload length validation
8. UTF-8 validation
9. JSON root detection
10. Versioned raw parse
11. Semantic validation
12. Normalization
13. Atomic persistence
```

#### 8.2.1 Input normalization

許可する前処理:

- UTF-8 BOM除去。
- 前後空白・改行除去。
- Markdown fenced code blockの外枠除去。
- 文字列全体を囲む一致した`'`、`"`、三連引用符の除去。

禁止する前処理:

- 文字列中間の空白や記号を無条件に削除。
- 不正文字の黙示置換。
- 欠落したBase64 padding以外の自動修復。
- 破損LZMAの部分結果採用。

#### 8.2.2 Base64

以下を識別する。

- Raw URL Encoding
- URL Encoding with padding
- Standard Raw Encoding
- Standard Encoding with padding

文字集合から候補を限定し、全候補の総当たりはしない。デコード後のLZMA-Aloneヘッダー妥当性で最終確認する。

#### 8.2.3 LZMA-Alone

`github.com/ulikunitz/xz/lzma`のReaderConfigを用い、dictionary capと出力上限を設定する。

- 圧縮入力上限: 8 MiB
- Dictionary上限: 64 MiB
- 展開出力上限: 64 MiB
- 展開予定サイズが既知の場合: 64 MiB以下であること
- 予定サイズと実出力サイズが一致すること
- CRC相当の保証が形式に存在しないため、後段の長さ・UTF-8・JSON・意味検証を必須とする

#### 8.2.4 MessagePack String envelope

対応型:

- fixstr: `0xA0..0xBF`
- str8: `0xD9`
- str16: `0xDA`
- str32: `0xDB`

対応しない型はエラーとする。MapやBinaryを「たぶんJSON」と推測しない。

必須検証:

```text
declared_string_length == remaining_uncompressed_bytes
```

末尾余剰byte、短いペイロード、オーバーフローを拒否する。

#### 8.2.5 JSON root detection

JSON Objectを一度RawMessage Mapへ読み、キーで判定する。

```text
Bundle root:
  has "profiles"
  optional "version"

Single profile root:
  has "inputs"
  has "id" or "name"
```

双方またはどちらでもない場合は曖昧・未知形式として拒否する。

### 8.3 Raw形式で確認済みの概念

正常サンプルのbundleには`version`と`profiles`があり、プロファイルには次が含まれていた。

- `id`
- `name`
- `inputs`
- `isSoftware`
- `isFavorite`
- `profileSettings`
- `errors`
- `version`

Inputでは次のカテゴリが確認された。

- `id`, `pinOne`, `pinTwo`
- `types`, `keyValues`, `metaValues`
- long/double用のtype、key、meta
- `macro`, `longMacro`, `doubleMacro`
- feature/double delay
- hold設定
- sequence trigger settings
- analog settings
- `label`
- turbo関連フィールド

形式世代により、値は数値文字列、JSON数値、`KeyW`、`AltLeft`等のsymbolic nameを取り得る。

### 8.4 Rawデータ型の方針

以下はIssue #20のmodel boundaryが満たす必須contractであり、現在のAPI availabilityを示すものではない。現在の実装状況はrepository READMEに従う。実装時は`profiledecode.Document`を受け取り、decoderの暫定kindを信頼せず、JSON rootを再判定しなければならない。`Parse`成功時の`RawExport`は`Bundle`か`Single`の一方だけをnon-nilとし、失敗時は常にzero valueを返して部分的なrootを返してはならない。

```go
type RawExport struct {
    Bundle *RawBundle
    Single *RawProfile
}

type RawBundle struct {
    Version  *RawScalar
    Profiles []RawProfile
    Unknown  map[string]json.RawMessage
}

type RawProfile struct {
    ID      *RawScalar
    Name    *RawScalar
    Version *RawScalar
    Inputs  []RawInput
    Unknown map[string]json.RawMessage
}

type RawInput map[string]json.RawMessage

type RawScalarKind uint8

const (
    ScalarNull RawScalarKind = iota
    ScalarBool
    ScalarString
    ScalarNumber
)

type RawScalar struct {
    Kind   RawScalarKind
    Raw    json.RawMessage
    Bool   bool
    String string
    Number json.Number
}
```

Absent scalarはnil pointer、存在する`null`はnon-nilの`ScalarNull`として区別する。空の配列はnon-nilかつ長さ0のsliceとして保持する。`RawScalar`はnull、bool、string、numberだけを受け入れ、numberをfloat64やint64へ狭めない。指数、小数、negative zero、int64を超えるnumberを含め、元のvalue tokenを`Raw`へ保持する。stringの`"001"`とnumberの`1`は別の値である。

Bundle、profile、inputのunknown valueはowned `json.RawMessage`として保持する。`RawInput`は全fieldをopaqueに扱い、input ID、binding、trigger、`macro`、`longMacro`、`doubleMacro`のschemaをIssue #20では推測しない。返却したraw bytesは入力`Document.JSON`から独立させる。Object key順序とvalue token外の無意味な空白はround-trip保証の対象外とする。

### 8.5 バージョンとsemantic admission

Rootまたはprofileの`version`は省略、null、bool、string、numberをraw evidenceとして保持できる。Objectまたはarrayなら`ERR_IMPORT_UNSUPPORTED_VERSION`とする。このerrorはversionの表現が非対応であることを示すだけで、scalar versionが対応世代であることを示さない。Issue #20はgeneration admission APIを提供しない。

Issue #40はprivacy-safeなversion-bearing binding evidenceとmacro grammarを確立した。Issue #42のadapterは、呼出側が明示する`SoftwareRelease="2.0.2"`と`SourceScope="azeron-software-export"`だけを受け付ける。閉じたTurbo/Macro/角度の設定行を正規化し、解釈前に1 macroあたり1,000 stepの上限を適用する。Issue #20はmacroをopaqueな値として構造制限の範囲で保持し、stepを数えない。raw `version`から世代を推測せず、未知世代を受け入れない。legacy numeric変換はv1後の[Issue #102](https://github.com/sh4869221b/azerlay/issues/102)で扱う。

### 8.6 セキュリティ制限

| 対象 | 上限 |
|---|---:|
| 入力文字列 | 16 MiB |
| Base64デコード後 | 8 MiB |
| LZMA dictionary | 64 MiB |
| LZMA展開後 | 64 MiB |
| プロファイル数 | 512 |
| 1プロファイルのinput数 | 256 |
| decoded単一文字列 | 65,536 UTF-8 bytes |
| JSON container nesting | 64 |
| 1マクロのstep数 | 1,000、Issue #42でsemantic interpretation前に検証 |

Model boundaryは全JSON token streamをmap構築前にscanする。Malformed JSON、invalid UTF-8、末尾の別JSON、全階層のdecoded duplicate key、root/kind不一致、field type不正は`ERR_IMPORT_ROOT`とする。`"a"`と`"\u0061"`は同じdecoded keyとして重複になる。Container depth 64とdecoded string 65,536 bytesは受理し、それぞれ65と65,537 bytesを`ERR_IMPORT_LIMIT_EXCEEDED`で拒否する。Unicode escapeの表記方法はdecoded byte数を変えない。

検証順序は、全streamのpreflight、root再判定と暫定kind一致、root version表現、profileとinputのshapeおよび件数とする。Profile内ではversion、id/name、inputsの順に検証する。Preflight中は最初に発見したfailureを返す。上限超過や他のerrorでは部分保存しない。Canonical synthetic rootの構造受理はSoftware generation supportを意味しない。

---

## 9. 内部データモデル

Raw Azeron JSONをUIや入力処理へ直接渡さない。正規化済みで版非依存のモデルを唯一の内部契約とする。

### 9.1 実装済みの順序付き中核モデル

Issue #42のowner承認済みoption Aでは、物理配置へ投影する前段として、次の版非依存モデルを実装した。

```go
type SourceMetadata struct {
    SoftwareRelease string
    SourceScope     string
}

type ProfileBundle struct {
    SchemaVersion int
    Source        SourceMetadata
    RootKind      RootKind
    Profiles      []Profile
    Raw           *RawBundleReference
}

type Profile struct {
    ID       *string
    Name     *string
    Controls []ControlBinding
    Raw      RawProfileReference
}

type ControlBinding struct {
    Label    *string
    Bindings []TriggerBinding
    Raw      RawBindingReference
}

type RawBindingReference struct {
    RootKind     RootKind
    ProfileIndex int
    InputIndex   int
    Fields       map[string]json.RawMessage
}

type TriggerBinding struct {
    Trigger           TriggerKind
    Kind              BindingKind
    Actions           []Action
    Turbo             *TurboBinding
    Macro             *MacroBinding
    Stick             *StickBinding
    TriggerDelayMS    *int
    TriggerIntervalMS *int
    ReleaseBehavior   *string
    Unknown           *UnknownBinding
}

type Action struct {
    Kind      ActionKind
    Code      CanonicalCode
    Modifiers []CanonicalCode
}
```

`Profiles`、`Controls`、`Bindings`、`Actions`はsource順を保つ。`ProfileIndex`と`InputIndex`はprovenanceであり、物理control、input ID、pinを表さない。重複IDや同じcanonical actionを持つcontrolも統合しない。`RawBundleReference`、`RawProfileReference`、`RawBindingReference`はownedかつopaqueな保持領域であり、後段がAzeron field名を読んでbinding semanticsを追加してはならない。

`RootKind`は`bundle`と`single`、`TriggerKind`は`single`、`long`、`double`、`unknown`を扱う。`BindingKind`は`keyboard`、`turbo`、`macro`、`stick`、`unknown`を区別する。`CanonicalCode`は閉じた行の`KEY_U`、`KEY_P`、`KEY_L`、`KEY_I`、`KEY_LEFTCTRL`、`KEY_T`と、Keyboard stick/Macro用の`KEY_W`、`KEY_A`、`KEY_S`、`KEY_D`を含む。`TurboBinding`はcodeと25/10回毎秒、`MacroBinding`はrepeatと順序付きのW Button 50 ms/Delay 100 ms、`StickBinding`はmodeとKeyboard方向、Xbox角度0/90を保持する。非0度のstickは入力座標へ投影しない。物理controlや未観測の実行動作はこのモデルから推測しない。

### 9.2 Canonical Code

Linuxの標準名を内部Canonical Codeとする方針は維持する。現在実装された変換は、Software 2.0.2の閉じたpredicateだけであり、symbol prefixや数値一致から変換を増やさない。

次は将来のcanonical vocabulary例であり、現在の対応表でも変換evidenceでもない。

```text
KEY_Q
KEY_LEFTALT
KEY_CAPSLOCK
BTN_LEFT
BTN_SIDE
BTN_GAMEPAD
ABS_X
ABS_Y
```

UI用表示は別層で`Q`、`Left Alt`等に整形する。

次のAzeron symbolic name変換も将来要件の例であり、実装済みの一般規則ではない。

```text
KeyW      → KEY_W
AltLeft   → KEY_LEFTALT
Digit1    → KEY_1
ArrowUp   → KEY_UP
```

完全な変換表は**要調査・リリース阻害**である。

### 9.3 Physical Control

```go
type PhysicalControl struct {
    ID             PhysicalControlID
    Model          DeviceModel
    Hand           Handedness
    AzeronInputIDs []int
    Group          ControlGroup
    Shape          Shape
    LabelAnchor    Point
    ZIndex         int
}
```

`PhysicalControlID`は位置を表す安定した識別子とする。

例:

```text
index.pull
index.push
index.upper
middle.pull
ring.pull
pinky.pull
thumb.stick
thumb.fiveway.up
thumb.fiveway.press
```

Azeronの`input ID`をそのまま公開API上の物理位置名にしない。モデル・左右・ファームウェア差をLayout Adapterで吸収する。

### 9.4 入力状態

```go
type InputSnapshot struct {
    Sequence     uint64
    Connected    bool
    Generations  Generations
    Availability Availability
    Controls     map[PhysicalControlID]PhysicalState // Known, Down: last observed
}

type OverlaySnapshot struct {
    Profile       *Profile
    Layout        *LayoutDefinition
    Device        DeviceStatus
    Controls      map[PhysicalControlID]VisualControlState
    Status        OverlayStatus
    Generation    uint64
}
```

これは概念モデルであり、入力の各物理controlは`Known`と`Down`を持つ最終観測状態とする。初期・切断・再open・検出した欠落や不正reportではunknownへ戻し、有効な通知はそのcontrolだけを更新する。未検出の末尾release欠落ではstaleが残り得る。実装のsnapshotは所有権を分離し、外部から変更させない。

Stickの方向・入力量は将来機能であり、このraw通知経路では未検証・未対応。描画は将来の`OverlaySnapshot`以外へアクセスしない。

---

## 10. プロファイルソース設計

### 10.1 インターフェース

```go
type ProfileSource interface {
    ID() SourceID
    Discover(ctx context.Context) ([]SourceDescriptor, error)
    Load(ctx context.Context, ref SourceRef) (*ProfileBundle, error)
    Watch(ctx context.Context, ref SourceRef) (<-chan SourceChange, error)
}
```

### 10.2 ImportedSource

- ユーザーが明示的に渡したエクスポートを処理する。
- 元データの正確なbytesをSHA-256名で保存する。hashはAzeron IDではない。
- opaque raw referenceを含む完全な正規化bundleとsource順の`pN.json` cacheを
  保存する。`pN`はone-based ordinalであり、Azeron IDではない。
- indexはcatalogとselected source/ordinalのauthorityである。初回importの
  provenance、時刻、decoder/normalizer revision、raw export versionを保持し、
  cache envelopeは現在のinterpretation revisionを持つ。parserまたはadapterの
  semantic変更時は該当revisionをbumpする。
- 同一bytesは入力経路に関係なくdeduplicateし、初回metadataを保持する。
- cacheが欠落、破損、または旧revisionなら、保存済みoriginalからmemory上で
  復元する。Discover、Load、showを含むread時にwriteやrepair、再selectionは行わない。
  indexまたはoriginalが破損している場合はstorage errorとする。

### 10.3 AzeronLocalSource

[保存済みJSONの安全読取契約](decisions/local-source.md)を参照。LocalSourceの製品実装は別Issueで行う。

必須機能だが、次が要調査である。

- Linux版Azeron Software 2.xの保存場所。
- Electron `userData`、IndexedDB、LevelDB、JSON等のどれを使用するか。
- プロファイル本体とactive profileの保存箇所。
- Software profileとon-board profileの区別。
- ファイル書換の原子性とロック方式。
- Azeron Software更新でのschema変化。

実装規則:

1. 既知パスを固定値1つにせず、検出器を版ごとに持つ。
2. 利用者が`local_store_path`を明示指定できる。
3. 読取専用で開く。
4. LevelDB等を使用する場合、稼働中プロセスのDBを直接開いて破損させない。安全なスナップショット取得方式を調査する。
5. 変更監視は親ディレクトリを監視し、rename置換にも追従する。
6. 読込失敗時はlast-known-goodを維持する。

### 10.4 ソース優先順位

既定:

```text
explicit selected source
    > detected Azeron local source
    > last selected imported source
    > most recent valid imported source
```

自動切替はユーザー設定で無効化できる。

### 10.5 永続化

原本と正規化キャッシュを分離する。

```text
$XDG_DATA_HOME/azerlay/
├── sources/<sha256>.azeron
├── profiles/<sha256>/
│   ├── bundle.json
│   └── p1.json ... pN.json
└── cache/
    ├── source-index.json
    └── import.lock
```

`XDG_DATA_HOME`がunset、empty、relativeの場合は`$HOME/.local/share`を使う。
application-owned directoryは0700、original、cache、index、lockを含むfileは
0600とする。inputとselectionを完全にvalidateした後に保存を開始し、indexを最後に
publishする。これにより通常の失敗は前のcatalogとselectionを保つ。commit後のoutput
failureは保存をrollbackしない。この保証はprocess-level atomic visibilityであり、
power loss後の全directory entryのdurabilityは保証しない。中断後のprivate orphanは
indexから参照されず、Discoverは無視する。

---

## 11. ゲームプロファイル

### 11.1 目的

Azeron側ラベルが空の場合に、割当キーからゲーム内アクション名を補完する。

### 11.2 形式

```toml
schema_version = 1
id = "bodycam"
name = "Bodycam"
locale = "ja-JP"

[bindings]
"KEY_Q" = "左リーン"
"KEY_E" = "右リーン / 使用"
"KEY_R" = "リロード"
"KEY_C" = "しゃがみ"
"KEY_LEFTSHIFT" = "スプリント"
"KEY_SPACE" = "ジャンプ / 乗り越え"

[controls]
"input:15:single" = "右リーン / インタラクト"
```

### 11.3 要件

- Built-inゲームプロファイルは`go:embed`で同梱する。
- ユーザープロファイルが同じIDなら上書きする。
- Canonical Binding単位とPhysical Control単位の両方を許可する。
- ゲーム側のキーバインドを自動改変しない。
- Bodycamの現行キー設定はゲーム版更新で変わり得るため、実ゲームのKey Bind画面で再確認する。**要調査・Bodycam同梱プロファイルのリリース阻害**。

---

## 12. デバイス検出と権限

### 12.1 検出と選択

sysfs `class/hidraw`、USB device親の`16d0:12f7:0111`、interface04
(`03/00/00`)と28-byte descriptor
`0601ff0a0101a1017508150026ff00954009018102954009029102c0`を照合する。
これはusage page `ff01`、usage `0101`、unnumbered64-byte Input/Outputである。
実際の読取・診断用openは`O_RDONLY|O_NONBLOCK|O_CLOEXEC`、fstatとOS保有の
HID identity/descriptor gettersおよび再取得したsysfs metadataで確認する。
診断はreportを消費せずcloseする。イベントノードは列挙対象・fallbackにしない。

USB device親ごとにgroupを作り、一意にadmittedなinterface04をcompleteとする。
再接続では既知serialが一意に一致する機器だけを選ぶ。serialなしは元USB親に
限定し、同じポートでの物理個体同一性までは保証しない。複数候補を勝手に選ばない。

### 12.2 権限

native packagingの`71-azerlay.rules`はhidraw subsystem、VID/PID/release、interface04限定の
uaccessとする。恒久input-group加入、root、VIDだけの広いgrantは要求しない。
Arch packageがruleをインストールし、manual archiveでは管理者が明示的に導入する。
アプリのCLIは権限を変更しない。uaccess自体がread-onlyを強制する
わけではなく、アプリのopen flagsと非書込契約で制限する。旧event-node ruleと
既存hostの広い0666 grantは新経路の権限検証ではない。

### 12.3 安全原則

- 一般キーボード、evdev、uinputを利用しない。
- grab/remap/inject、HID write、Feature/Output/GET_INPUT要求を行わない。
- 公式Azeron SoftwareのSOFTWARE modeを明示的前提とし、アプリ単独初期化しない。
- USB identityからhand、Software mode、Hardware revisionを推定しない。

---

## 13. 入力処理

### 13.1 ReaderとReducer

選択したhidrawに1 reader、順序付きreducerを使う。ボタンごとのgoroutineや
汎用backend frameworkを作らない。vendor typeはbyte2=57、counterはbyte3、
宣言payload lengthはbyte6=2、source/stateはbyte7/8である。64-byte reportの
確認済み30 IDsとstate0/1だけを受理し、layout.LoadEmbeddedのsource→region対応を
使う。stick.mainは未対応。他typeは無視し、paddingは解釈しない。

### 13.2 最終観測状態

Snapshotは**最終観測状態**であり、連続した現在状態ではない。
初期/reopenは全unknown・availability unconfirmed。有効reportでそのcontrolだけを
knownにし、availability observedとする。不正物理reportは全unknownに戻す。
counterの通常増分以外（wrap/resetを含む）は保守的なloss疑いとして過去の知識を
破棄する。counterから無欠落・一周・連続性や完全復旧を保証しない。
**検出できない末尾release欠落では古い観測が無期限に残り得る。**
沈黙やtimeoutをrelease/unavailableの根拠にしない。

読取失敗/disconnectは全unknown・unavailable。Managedは選択identityを保持し、
250msごとに再取得する。旧reader/fdをclose/joinしてから新sessionを開始し、
成功した置換でのみDevice generationを増やす。Profile generationと設定/profileの
last-good状態を維持する。回復は新たな個別通知のみで、状態snapshot要求は行わない。

### 13.3 表示への境界

静的profile/layoutと出力候補は割当表示用に保持し、入力元の決定には使わない。
ProjectMatchingは明示されたleft-hand Cyborg II、Software2.0.2、表示firmware111、
unknown revision、keyboard-stick layoutおよびSOFTWARE modeで30領域を投影する。
SOFTWARE modeとlayoutのkeyboard-stickは別条件である。非対応contextはunknown。
analog/live出力キー/long・double・macro発火完了は未対応のままとする。
入力libraryからrun/controller/GTKへの新規配線は今回行わない。今後のUI接続でも
GTK main thread上の描画はI/Oや待機を行わず、最新immutable snapshotを使う。

## 14. レイアウトと描画

### 14.1 レイアウト定義

公式画像ではなく、独自のベクター模式図をデータ駆動で描画する。

```go
type LayoutDefinition struct {
    SchemaVersion int
    Model         DeviceModel
    Hand          Handedness
    ViewBox       Rect
    Controls      []PhysicalControl
    Decorations   []Shape
}
```

レイアウトファイルはJSONとし、組込みassetsとして同梱する。

### 14.2 図形

最低限のprimitive:

- rounded rectangle
- polygon
- circle / ellipse
- line / path
- text anchor
- group transform

Cairoで描画し、GPU専用APIや独自レンダリングエンジンを導入しない。

### 14.3 表示情報

各Physical Controlは次を表示できる。

```text
┌──────────────────┐
│ 左リーン          │  ← resolved action label
│ Q                 │  ← canonical binding display
│ HOLD / DOUBLE     │  ← trigger badges when applicable
└──────────────────┘
```

すべてを常時表示すると混雑する場合、表示密度を設定する。

- `compact`: アクション名のみ
- `normal`: アクション + キー
- `detailed`: トリガー、マクロ、曖昧性も表示

### 14.4 視覚状態

色そのものをロジックへ埋め込まずSemantic Tokenを用いる。

```text
surface.background
control.idle
control.pressed
control.held
control.double
control.macro
control.ambiguous
control.unbound
control.unknown
text.primary
text.secondary
status.error
status.warning
```

テーマは明暗、高コントラストを持つ。色だけで状態を表現せず、枠、太さ、記号、ラベルを併用する。

### 14.5 GTK4 / Cairo

- `GtkDrawingArea`にdraw functionを設定する。
- 状態変化時だけ`QueueDraw`する。
- 文字計測と省略を描画前に行う。
- レイアウト座標をウィンドウサイズへ一様変換する。
- draw callback内でI/O、JSON解析、ロック待ちを行わない。

### 14.6 Layer Shell

Go向けに古いGTK3用bindingを流用しない。`gtk4-layer-shell`のC APIへ、必要関数だけを公開する薄いCGo bridgeを作る。

必要な操作:

- initialize for GTK window
- set namespace
- set layer = overlay
- set anchor
- set margin
- set monitor
- set keyboard mode = none
- set exclusive zone = `-1`（v1のHyprland配置では排他的領域を確保しない）

CGoは`internal/layershell`へ隔離し、他パッケージへ`C.*`型を漏らさない。

### 14.7 Click-through

Layer Shellのkeyboard modeだけではポインター透過を保証しない。実装は
GTK/GDK surfaceがmapされた後に空のinput regionを設定し、surfaceの再生成や
placement更新後にも適用する。keyboard modeは`none`のまま維持する。
GDKの設定APIは成否を返さないため、statusの
`overlay.input_region_applied`は適用呼び出しの状態であり、Compositorの
acknowledgementやポインター配送そのものを示さない。requested visibilityと
実際のmapping状態は別々にstatusへ出す。show/hide/toggleの応答は要求受付を
示し、GTK main threadでのmappingは非同期に進む。

2026-09-26のHyprland QAでは、アプリ側で空のinput regionを設定した状態で
pointer motion、click、scrollが下側のGTK receiverへ届き、同じ重なり位置で
full input regionに切り替えた対照ではpointer deliveryが止まることを確認した。
Hyprland v1の実測詳細と範囲は[overlay troubleshooting](troubleshooting.md#overlay-click-through-and-monitor-recovery)を参照。
Swayは隔離されたnative test fixtureに限り、製品適合の根拠にしない。Niriはv1対象外。

### 14.8 Direct Scanout

Compositorによってはoverlay surfaceが存在するとDirect Scanoutを無効化し、性能、VRR、遅延、消費電力へ影響する可能性がある。

- v1対象のHyprlandで実測する。NiriとGamescopeの実測はv1後の別課題とする。
- オーバーレイ非表示時はsurfaceをunmapまたは破棄し、Direct Scanout復帰可能性を高める。
- 単にopacity 0にするだけで隠さない。
- 影響をREADMEへ明記する。

これは**要調査・性能リリース阻害**。

---

## 15. プロセス・並行処理設計

### 15.1 単一プロセス

Azerlayは原則1プロセスで動作する。

```text
azerlay process
├── GTK main thread
├── device manager
├── qualified hidraw reader
├── input reducer
├── source watcher
├── config watcher
└── control socket server
```

別デーモンとGUIクライアントに分割しない。

### 15.2 GTKスレッド

`main()`の初期段階で`runtime.LockOSThread()`を呼び、GTK操作をmain OS threadに限定する。

バックグラウンドgoroutineからGTKオブジェクトを直接触らない。メインコンテキストへ最小通知を投げ、最新immutable snapshotを読む。

### 15.3 状態公開

以下のいずれかを採用する。

- `atomic.Pointer[OverlaySnapshot]`
- 容量1のlatest-only channel

推奨は、Reducerがimmutable snapshotを生成し、`atomic.Pointer`へ差し替える方式。GTK側はdraw開始時に1回loadする。

### 15.4 世代管理

設定、プロファイル、レイアウト、デバイスごとにgenerationを付与する。古い非同期結果を新状態へ上書きしない。

```text
Config generation: 12
Profile load started at generation 12
Config changes to generation 13
Old load result arrives
→ discard because generation mismatch
```

### 15.5 終了

`context.Context`で全コンポーネントを停止する。終了順序:

1. 新規制御要求停止
2. watcher停止
3. device reader停止・fd close
4. reducer停止
5. socket削除
6. overlay unmap
7. GTK終了

---

## 16. 設定設計

### 16.1 パス

```text
$XDG_CONFIG_HOME/azerlay/
├── config.toml
├── games/
│   └── bodycam.toml
└── layouts/
    └── custom-*.json

$XDG_DATA_HOME/azerlay/
├── sources/
├── profiles/
└── cache/

$XDG_STATE_HOME/azerlay/
└── last-good.json

$XDG_RUNTIME_DIR/azerlay/
├── control.sock
└── instance.lock
```

環境変数未設定時はXDG規定の既定パスへフォールバックする。

### 16.2 `config.toml`例

```toml
schema_version = 1

[device]
model = "cyborg-ii"
hand = "left"
serial = ""
auto_reconnect = true

[input]
# Legacy accepted setting; unwired. The input library never starts evdev.
source = "auto"
refresh_hz = 60
show_ambiguous = true

[profile]
source = "auto"
selected_id = ""
game = "bodycam"
watch = true

[overlay]
monitor = "DP-2"
anchor = "bottom-right"
margin_x = 24
margin_y = 24
scale = 1.0
opacity = 0.88
mode = "normal"
show_profile_name = true
show_status = true
show_unbound = false

[appearance]
theme = "dark"
font_scale = 1.0
high_contrast = false

[diagnostics]
log_level = "info"
include_bindings = false
```

値の上限・下限を検証する。未知キーは警告し、将来追加キーを破壊しない。

### 16.3 原子的再読込

```text
read files
  ↓
parse all
  ↓
validate cross references
  ↓
build candidate snapshot
  ↓
atomic swap
```

途中失敗時は既存状態を維持する。

### 16.4 fswatcher

`github.com/fswatcher/fswatcher`を使用する。ファイル自身ではなく親ディレクトリを監視し、atomic renameによる置換へ対応する。監視登録時は`Create`、`Write`、`Remove`、`Rename`のうち必要なイベントだけを指定する。イベントは100〜300ms程度debounceし、安定化確認後に読む。具体値は実測で確定する。

1.0では再帰監視を必須としない。Azeron LocalSourceの調査結果、または将来の階層化されたgame profile/layout監視で必要と判明した場合のみ`AddRecursive`を使用する。

---

## 17. CLI設計

### 17.1 コマンド一覧

```text
azerlay run
azerlay import <file|->
azerlay validate <file|->
azerlay profiles list
azerlay profiles select <id|name>
azerlay profiles show [--json]
azerlay devices list
azerlay devices inspect [path]
azerlay show
azerlay hide
azerlay toggle
azerlay reload
azerlay status
azerlay doctor
azerlay dump-profile [--raw|--normalized]
azerlay quit
azerlay version
```

#### 17.1.1 `devices` (implemented)

```text
azerlay devices --help
azerlay devices list [--json] [--help]
azerlay devices inspect [path] [--json] [--help]
```

These standalone commands collect metadata and probe admitted hidraw nodes
read-only for access and identification. They do not consume HID reports, change
permissions, or use configuration, profile storage, or the runtime socket.
Doctor uses the same passive identity/access boundary. Runtime input remains unwired to run/GTK.
Admission and grouping follow the researched `16d0:12f7:0111`
[identity contract](decisions/device-identity.md#identity-and-grouping-contract);
no wider variant or release support is implied.

Flags are unique literal booleans: `--json=true`, duplicate flags, flags before
the subcommand, list positionals, and more than one inspect path are usage
errors. `--` ends flag parsing; a following path is literal. Valid help returns
text and exit `0` without discovery, including with `--json`; invalid syntax
still fails before discovery. For usage errors, a literal `--json` before `--`
selects JSON even if other arguments are invalid.

List returns admitted nodes and groups plus relevant candidate diagnostics.
Inspect without a path also shows excluded relevant candidates; explicit
inspect examines only the supplied node. Relative paths and symlinks must map
by device number to an existing hidraw node in sysfs. Explicit inspection does
not bypass admission or open sibling nodes. Paths are transient locators.

| Exit | Meaning |
| --- | --- |
| `0` | At least one requested admitted node is readable and no error diagnostic exists. |
| `1` | No qualifying node, unsupported explicit target, metadata/access/disconnection error, or output failure. Available partial results are retained. |
| `2` | Invalid command arguments (`ERR_CLI_USAGE`). |

Warnings alone do not fail a qualifying readable subset. Unsupported automatic
candidates use warning-level `ERR_DEVICE_UNSUPPORTED`; explicit unsupported
targets use error-level severity. Multiple admitted interface04 nodes produce `ERR_DEVICE_AMBIGUOUS`. Candidate metadata failures, denied access, and
disconnections remain errors even when another node is readable. Root adds
`WARN_DEVICE_ROOT` without changing the exit status: root access does not prove
ordinary-user access. A successful open does not verify installed least-privilege
ACLs; see [permission troubleshooting](troubleshooting.md).

Text results and warnings go to stdout, errors to stderr. Device-supplied control
characters are escaped. JSON is one newline-terminated object on stdout, with
no stderr when writing succeeds, including usage failures:

```text
{schema_version: 2, command, ok, result, error}
```

`command` is `devices list` or `devices inspect` (`devices` for parent usage).
`ok` corresponds to successful discovery. `result` contains `groups`, `nodes`,
and `diagnostics` arrays, including empty arrays; it is `null` when usage,
resolution, or enumeration fails before a result exists. `error` is `null` or
the first error diagnostic in output order. Diagnostics contain `code`,
`severity`, `stage`, `summary`, nullable `target`, and `remediation`.

Groups contain `usb_parent`, `complete`, and `hidraw_paths`. Completeness means
exactly one interface04 was admitted, independently of read access.
Node summaries contain `path`, nullable `usb_parent` and `interface`, `roles`,
`admission` (`admitted`, `unsupported`, `indeterminate`), and `access`
(`readable`, `denied`, `unavailable`, `not_checked`). Inspect adds `name`,
`usb_id`, `sysfs_path`, `physical_path`, **`serial`**,
`interface_descriptor`, and `report_descriptor`; list omits all these detail fields.
Inspect may disclose serial even without a path. Unknown optional values are
`null`. USB/interface IDs use lowercase fixed-width hexadecimal strings.
Report descriptor summary contains numeric `usage_page`, `usage`, `report_bytes`
and boolean `numbered`; raw descriptor bytes are never output. The obsolete
`input_id` and `capabilities` fields are absent. Groups sort by parent path, nodes by
hidraw number/path, and diagnostics by target then code.

### 17.2 `run`

```text
azerlay run [--config PATH] [--foreground]
```

- 既存インスタンスがある場合は二重起動せずstatusを返す。
- Waylandでない場合は明確に終了する。
- 有効プロファイルがなくても、初期案内状態として起動可能。

### 17.3 `import`

```text
azerlay import [--json] [--software-release RELEASE] [--profile-index N] {FILE|-|--text PAYLOAD}
azerlay validate [--json] [--software-release RELEASE] {FILE|-|--text PAYLOAD}
azerlay profiles show [--json]
```

current admissionはexactly Software 2.0.2である。`validate`はread-onlyで、
`import`は成功時にoriginalとselectionを保存する。複数profileではone-based
`--profile-index N`が必要であり、inputを再度渡す。`profiles show`はinputなしで
保存済みselectionを表示し、cache recoveryもmemory内だけで行う。storage failureは
`ERR_PROFILE_STORAGE`/`storage`、selectionなしは`ERR_PROFILE_NOT_FOUND`/`selection`、
showの不正なsyntaxは`ERR_CLI_USAGE`/`usage`である。

### 17.4 `doctor`

```text
azerlay doctor [--config PATH] [--json] [--include-bindings] [--help]
```

`--config=PATH`も受け付ける。位置引数、重複flag、空のconfig値、未知のflag、
boolean flagの`=value`形式は拒否する。有効なsyntaxの`--help`は検査せず終了0。
設定とprofileは次回起動時の選択を読み取りで診断し、稼働中processは不要である。
live socketは別に検査し、session-onlyのprofile選択で設定済み選択を置き換えない。

通常のreportは次の全カテゴリをこの順序で含み、同カテゴリ内は検査の追加順を保つ。
独立した検査は失敗後も続行する。

```text
session
libraries
layer-shell
monitor
configuration
profile-source
device-discovery
permissions
hidraw-capabilities
control-socket
```

| severity | 終了コード |
| --- | --- |
| `ok` | `0` |
| `warning` | `1` |
| `error` | `2` |
| `internal_error` | `3` |

reportの終了コードは最大severityから決める。現在のbuildではlibraries、layer-shell、
monitorが未実装で、
`WARN_CHECK_NOT_IMPLEMENTED`になる。他の問題がない環境でも終了1であり、終了0や
overlayの動作可能性を保証しない。sessionの成功は`WAYLAND_DISPLAY`が非空という
environment確認だけで、compositor接続を検証しない。

device-discoveryは対応groupの有無/曖昧性、permissionsはread-only open可否、
hidraw-capabilitiesはdescriptor対応を検査する。reportは消費せず機器へrequestを
送らない。成功は通知初期化を保証せず、公式SoftwareのSOFTWARE mode確認を案内する。
serial/raw reportsを出さず、安全なnode targetと固定診断のみを投影する。

主要な診断codeと分類:

| 状態 | code / severity |
| --- | --- |
| 対応deviceなし・descriptor非対応 | `ERR_DEVICE_NOT_FOUND`・`ERR_DEVICE_UNSUPPORTED` / warning |
| device曖昧・権限拒否・metadata失敗・切断 | 対応する`ERR_DEVICE_*` / error |
| Wayland環境なし | `ERR_SESSION_WAYLAND_REQUIRED` / error |
| 設定なし・無効 | `ERR_CONFIG_NOT_FOUND`・`ERR_CONFIG_INVALID` / error |
| 設定の未知key | `WARN_CONFIG_UNKNOWN_KEY` / warning（key名や値を出さず集約） |
| 設定失敗によるprofile検査不可 | `WARN_CHECK_SKIPPED` / warning |
| profile未選択・不一致・曖昧 | `ERR_PROFILE_NOT_FOUND`・`ERR_PROFILE_AMBIGUOUS` / warning |
| local source未実装 | `ERR_PROFILE_SOURCE_UNAVAILABLE` / warning（#22待ち、fallbackなし） |
| profile storage不正・読み取り不可 | `ERR_PROFILE_STORAGE` / error |
| 設定・store・socketの権限拒否 | `ERR_DIAGNOSTIC_PERMISSION` / error（permissionsカテゴリ） |
| socket未起動・dial時の接続拒否 | `WARN_CONTROL_NOT_RUNNING`・`WARN_CONTROL_STALE` / warning |
| runtime・socketの安全性または通信失敗 | 対応する`ERR_CONTROL_*` / error |
| 未知のlive degraded code | `WARN_CONTROL_DEGRADED` / warning |
| 構文エラー | `ERR_CLI_USAGE` / error（commandカテゴリ1件だけ） |
| 内部診断エラー | `ERR_DOCTOR_INTERNAL` / internal_error |

成功codeは`OK_SESSION_ENVIRONMENT`、`OK_CONFIGURATION`、`OK_PROFILE_SOURCE`、
`OK_CONTROL_SOCKET`、`OK_DEVICE_DISCOVERY`、`OK_DEVICE_ACCESS`、
`OK_HIDRAW_CAPABILITIES`。socket成功もconnectivityだけを表し、live degraded reasonsを
省略しない。既知のdevice/input/renderer、profile、configのcodeだけを固定文言へ写像し、
同じlive reason codeを重複表示しない。liveのconfig failureとprofile storage failureはerror、
その他の既知degraded状態はwarningとする。remoteのreason/name/source_refや
underlying errorはそのまま出力しない。timeoutや不正応答はstaleと判定しない。

`--json`は改行終端のobjectをstdoutへ1件出す。トップレベルは`schema_version: 2`、
`command: "doctor"`、`exit_code`、`checks`で、既存control reportの`ok`/`result`/`error`
envelopeは使わない。各checkは`category`、`code`、`summary`、`target`、`remediation`、
`severity`を持つ。`target`は常に存在するstringまたはnullで、他はstringである。
non-okの結果には固定の修正手順を含める。targetのpathは表示を許可しており、
匿名化出力ではない。textではpathやlabelをquoteして制御文字をescapeする。

`profile_details`は、その呼び出しの`--include-bindings`があり、設定済みprofileの
解決が成功した場合だけ付ける。永続設定`diagnostics.include_bindings`では許可しない。
内容はone-basedの`profile_index`とsource順の`controls`。各controlはone-basedの
`input_index`、stringまたはnullの`label`、`bindings`を持つ。bindingのfieldは
`trigger`、`kind`、`actions`（各actionの`kind`、`code`、`modifiers`）、
`trigger_delay_ms`、`trigger_interval_ms`、`release_behavior`、`unknown_reason`。
未設定のtiming/release/reasonはnullで、unknown/macroは固定の正規化済みreasonに留める。
raw export・raw field・macro内容・profile ID/name・storage hash・source-origin path・
event streamはflag付きでも出さず、非選択profileのcontrolも含めない。

text診断はstdout、text構文エラーはstderrへ出す。JSONは構文エラーも上記の専用reportで、
出力成功時のstderrは空。書き込み失敗は終了3とし、途中まで出たJSONへ別objectを追加しない。
設定・保存データ・稼働状態を変更せず、directory/lock作成、socket削除、自動修復も行わない。
例えば設定なしの`azerlay doctor --json`は`ERR_CONFIG_NOT_FOUND`で終了2となり、
`schema_version = 1`を含む有効な設定の作成を案内する。

### 17.5 機械可読出力

主要コマンドで`--json`をサポートする。JSON schema versionを含める。

---

## 18. 制御ソケット

### 18.1 目的

v1ではHyprlandのキーバインドやスクリプトから、動作中Azerlayを即時操作する。Niriでの連携はv1後に検証する。

### 18.2 プロトコル

Unix Domain Socket上のJSON Lines、protocol version 1。
実装済みの契約とCLIの提供範囲は[control-protocol.md](control-protocol.md)を参照。

Request例:

```json
{"version":1,"id":"42","method":"overlay.toggle","params":{}}
```

Response例:

```json
{"version":1,"id":"42","ok":true,"result":{"visible":false},"error":null}
```

### 18.3 セキュリティ

- socket mode 0600
- runtime directory mode 0700
- LinuxのSO_PEERCREDで双方の同一UIDを必須検証
- request最大64 KiB
- 1接続あたりtimeout
- 任意ファイル読込やshell実行メソッドを公開しない

---

## 19. エラー設計

### 19.1 安定したエラーコード

例:

```text
ERR_CONFIG_NOT_FOUND
ERR_CONFIG_INVALID
ERR_IMPORT_ENCODING
ERR_IMPORT_LZMA_HEADER
ERR_IMPORT_LZMA_CORRUPT
ERR_IMPORT_MSGPACK_TYPE
ERR_IMPORT_LENGTH_MISMATCH
ERR_IMPORT_UTF8
ERR_IMPORT_JSON
ERR_IMPORT_UNSUPPORTED_VERSION
ERR_IMPORT_LIMIT_EXCEEDED
ERR_PROFILE_NOT_FOUND
ERR_DEVICE_NOT_FOUND
ERR_DEVICE_PERMISSION
ERR_DEVICE_DISCONNECTED
ERR_DEVICE_UNSUPPORTED
ERR_DEVICE_METADATA
ERR_LAYER_SHELL_UNAVAILABLE
ERR_MONITOR_NOT_FOUND
ERR_INSTANCE_RUNNING
```

### 19.2 表示原則

エラーは次を含む。

- 安定したcode
- 利用者向けsummary
- 技術details
- 対象path/device
- remediation
- underlying cause chain

秘密やプロファイル内容を既定ログへ出さない。

### 19.3 Last-known-good

次の再読込失敗は動作継続可能とする。

- `config.toml`編集中の一時構文エラー
- ローカルAzeron store書換途中
- game profile不正
- layout上書き不正

現在の正常状態を保持し、statusにdegraded reasonを載せる。

---

## 20. セキュリティ・プライバシー

### 20.1 脅威

- 悪意ある圧縮データによるメモリ枯渇。
- 巨大JSON、深いnest、巨大macroによるDoS。
- hidraw権限の過剰付与。
- 全キーボード入力監視による機密情報取得。
- 制御socketの他ユーザー操作。
- AzeronローカルDBの破損。
- ログへの個人用ラベル・入力内容流出。

### 20.2 対策

- 圧縮・展開・JSON・配列・文字列の上限。
- Azeron DeviceGroupだけを監視。
- root、`input`グループ、grab、uinputを不使用。
- udevで対象hidraw・VID/PID/release・interface04だけに`uaccess`。
- socket 0600、同一UID確認。
- ローカルAzeronデータはread-only。
- データ保存0600、ディレクトリ0700。
- ネットワーク通信・テレメトリーなし。
- shell-outなし。
- インポートのfuzz testing。
- panicをプロセス境界で回収するだけでなく、parserをpanic-freeにする。

### 20.3 プライバシー

Azerlayは選択した確認済みinterface04 hidrawのみを読み、一般キーボードやevdevを監視しない。raw report全体、serial、private profileや入力streamをログ・永続保存しない。明示された実機検証では対象source/state・件数・最小の相対時刻と判定だけを記録する。

---

## 21. 非機能要件

### 21.1 性能

ターゲット実機上の目標値:

| 指標 | 目標 |
|---|---:|
| キーevent→ハイライト p95 | 20 ms以下（60 Hz設定） |
| cold start | 1秒以下 |
| 通常インポート | 250 ms以下 |
| idle CPU | 1 core換算0.5%以下 |
| 60 Hz描画中CPU | 1 core換算2%以下 |
| RSS | 120 MiB以下 |
| disconnect→reconnect復帰 | 2秒以内 |
| 入力report loss | 検出時unknown。未検出末尾欠落のstale状態は制約として許容 |

GTK、font cache、環境差があるため、CIだけでなくターゲット実機で測定する。

### 21.2 信頼性

- 不正インポートでpanicしない。
- hidraw切断でプロセス終了しない。
- 設定再読込失敗で正常状態を失わない。
- 起動中のAzeron Softwareを妨害しない。
- 24時間連続実行でgoroutine、fd、メモリが増え続けない。

### 21.3 可用性

- オーバーレイがなくてもCLI診断・インポートが使える。
- Profile source障害時はlast-known-goodで表示継続。
- 明示指定したmonitorの消失時は他出力へ移動せず、状態を保持して非表示にする。同じ設定キーが一意に再解決された場合のみ表示を再開する。物理的に同じmonitorの復帰は保証しない（[実測と未検証範囲](decisions/compositor-placement.md)）。

### 21.4 保守性

- Raw形式、正規化、入力、描画を明確に分ける。
- CGoは1パッケージへ限定。
- OS依存コードは`*_linux.go`へ限定。
- バージョン変換表にgolden testを置く。
- 不要なDI framework、ECS、Clean Architecture層を導入しない。

### 21.5 アクセシビリティ

- font scale
- high contrast
- opacity
- ラベル表示密度
- 色以外の状態表現
- 左右モデル
- 日本語フォントfallback

---

## 22. 技術スタック

### 22.1 採用

| 分野 | 採用技術 | 備考 |
|---|---|---|
| 言語 | Go 1.27.x | patch版をCIで固定 |
| Linux入力 | 標準os/syscall + 限定hidraw decoder | 受動readとOS保有metadata getterのみ |
| GUI | GTK4 + `github.com/diamondburned/gotk4` | branch 4系をcommit/pseudo-version固定 |
| Overlay | `gtk4-layer-shell` 1.3.x | C APIへ薄いCGo bridge |
| 描画 | Cairo / GTK4 DrawingArea | gotk4経由 |
| LZMA | `github.com/ulikunitz/xz/lzma` | ReaderConfigで上限設定 |
| JSON | `encoding/json` | `UseNumber`、RawMessage |
| TOML | `github.com/pelletier/go-toml/v2` | config/game profile |
| File watch | `github.com/fswatcher/fswatcher` | parent directory監視、event mask指定 |
| Hash | `crypto/sha256` | import source identity |
| Assets | `embed` | layouts、built-in game profiles |
| IPC | `net.UnixConn` | JSON Lines v1 |
| 権限 | udev `TAG+="uaccess"` | 対象機器限定 |
| サービス | systemd user unit | 任意の自動起動方法 |

### 22.2 条件付き依存

入力移行ではproduction dependencyを追加しない。既存標準ライブラリの
os/syscallとGo pollerでread-only lifecycleを実装する。

### 22.3 CGo方針

`CGO_ENABLED=1`を必須とする。配布バイナリはglibc、GTK4、gtk4-layer-shell等のシステム共有ライブラリへ依存する。

本番bridgeは`internal/layershell`へ隔離し、`Init`/`Apply`とsurface取得をGoのGTK/GDK wrapperで公開する。C型を公開APIへ出さない。surfaceのtransfer-none参照はgotk4の所有権管理で保持し、動的marshalerのuintptr往復を避ける。gtk4-layer-shellをWayland client libraryより先にロードする。

GTK ownerの生成・設定適用・破棄はlocked main OS threadで行う。`run`は起動時にhidden windowを作り、CLI visibility要求を同じthreadへ通知して実際のmappingを切り替える。描画とdevice inputのruntime統合は未実装。

### 22.4 非採用

- Electron / Tauri / WebView
- React / TypeScript
- wgpu / Vulkan / OpenGLの独自描画
- SDL / GLFW
- libinput
- uinput
- hidapi（1.0通常経路）
- DBus（必須IPCとして）
- SQLite
- ネットワークサーバー
- daemon/clientの別バイナリ化
- goroutine-per-button
- フレームごとのJSON/TOML解析

---

## 23. リポジトリ構成

```text
azerlay/
├── cmd/
│   └── azerlay/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   └── lifecycle.go
│   ├── cli/
│   │   ├── root.go
│   │   ├── import.go
│   │   ├── doctor.go
│   │   └── control.go
│   ├── config/
│   │   ├── model.go
│   │   ├── load.go
│   │   ├── validate.go
│   │   └── watch.go
│   ├── azeron/
│   │   ├── model.go
│   │   ├── canonical.go
│   │   └── export/
│   │       ├── decode.go
│   │       ├── base64.go
│   │       ├── lzma.go
│   │       ├── msgpack_string.go
│   │       ├── root.go
│   │       ├── raw.go
│   │       ├── adapter.go
│   │       └── normalize.go
│   ├── profile/
│   │   ├── source.go
│   │   ├── imported.go
│   │   ├── local.go
│   │   ├── select.go
│   │   └── store.go
│   ├── device/
│   │   ├── discover.go
│   │   ├── identity.go
│   │   ├── sysfs.go
│   │   ├── group.go
│   │   └── permission.go
│   ├── input/
│   │   ├── reader.go
│   │   ├── physical_report.go
│   │   ├── reducer.go
│   │   ├── snapshot.go
│   │   ├── axis.go
│   │   └── matching.go
│   ├── layout/
│   │   ├── model.go
│   │   ├── load.go
│   │   ├── map.go
│   │   └── validate.go
│   ├── overlay/
│   │   ├── window.go
│   │   ├── state.go
│   │   ├── render.go
│   │   ├── text.go
│   │   └── theme.go
│   ├── layershell/
│   │   ├── bridge_linux.go
│   │   ├── bridge.h
│   │   └── bridge.c
│   ├── control/
│   │   ├── protocol.go
│   │   ├── server.go
│   │   └── client.go
│   └── diagnostics/
│       ├── doctor.go
│       ├── result.go
│       └── checks.go
├── assets/
│   ├── layouts/
│   │   ├── cyborg-ii-left.json
│   │   └── cyborg-ii-right.json  # v1後の対応。v1の必須assetではない
│   ├── games/
│   │   ├── bodycam.ja-JP.toml
│   │   └── bodycam.en-US.toml
│   └── themes/
├── packaging/
│   ├── arch/
│   │   └── PKGBUILD
│   ├── systemd/
│   │   └── azerlay.service.in
│   ├── udev/
│   │   └── 71-azerlay.rules
│   └── desktop/
│       └── io.github.@OWNER@.azerlay.desktop.in
├── docs/
│   ├── architecture.md
│   ├── profile-format.md
│   ├── troubleshooting.md
│   ├── compatibility.md
│   └── security.md
├── testdata/
│   ├── export/
│   ├── config/
│   └── layouts/
├── go.mod
├── go.sum
├── LICENSE
├── NOTICE
├── CHANGELOG.md
└── README.md
```

`pkg/`は外部利用を約束するAPIが必要になるまで作らない。まず`internal/`に閉じる。

---

## 24. ビルド・配布

### 24.1 ビルド要件

Arch/CachyOS例:

```bash
sudo pacman -S --needed \
  go \
  gcc \
  pkgconf \
  gtk4 \
  gtk4-layer-shell \
  gobject-introspection
```

native test fixtureには追加で`sway`と`dbus`が必要。非rootで`scripts/test-wayland.sh go test -race -shuffle=on -count=1 ./...`を実行する。SwayはCI用protocol fixtureであり、製品サポートの確認ではない。Ubuntuの導入・build手順はv1後のIssue #98で扱う。

### 24.2 ビルド

```bash
CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags '-X main.version=0.0.0' -o build/azerlay ./cmd/azerlay
```

現在のpublic version項目はversionだけであり、commit・build dateは追加しない。
Go 1.27.xとnative依存が必要。配布用の実際の手順は`scripts/package.sh VERSION OUTPUT_DIR`を使う。

### 24.3 配布形式

現在はローカルsource/binary tarballとlocal-source Arch PKGBUILDを提供する。
PKGBUILDのchecksum placeholderは一時recipeで通常の`updpkgsums`により置換する。
未公開Release URLや`SKIP`は使わない。GitHub Release/AUR公開、deb/rpmは今回の範囲外。
license grantとdesktop ownerは未確定であり、local smoke packageも再配布許諾ではない。

GTK4等を内包した巨大なAppImageは初期の必須配布形式としない。Layer Shell、GObject、フォント、Wayland統合でDistribution側ライブラリとの相性確認が必要なためである。

### 24.4 systemd user service

```ini
[Unit]
Description=Azerlay
After=graphical-session.target
PartOf=graphical-session.target

[Service]
Type=simple
ExecStart=@BINDIR@/azerlay run --foreground
Restart=on-failure
RestartSec=2

[Install]
WantedBy=graphical-session.target
```

templateの`@BINDIR@`は実際のabsolute binary directoryへ置換する。Arch packageは`/usr/bin`。
serviceは自動enable/startしない。manual startは到達可能なWayland displayと利用者所有の
runtime directory、user managerへの環境importを前提とする。将来の自動起動をenableするのは
compositorがactiveな`graphical-session.target`を管理する場合に限る。targetを管理しない
plain Hyprlandの自動起動はforeground `exec-once`を代替とする。具体的な導入・削除はREADME参照。

### 24.5 リリース成果物

- binary archive
- SHA-256 checksums
- SBOM（SPDXまたはCycloneDX）
- dependency license report
- CHANGELOG
- compatibility matrix
- udev rule
- systemd user unit
- detached signatureまたはSigstore署名を推奨

---

## 25. テスト設計

### 25.1 Unit Test

対象:

- wrapper文字列正規化
- Base64 variant判定
- LZMA header、dict cap、output cap
- MessagePack fixstr/str8/str16/str32
- 長さ一致・不一致
- UTF-8検証
- bundle/single root判定
- RawScalar
- version adapter
- Canonical Code変換
- label優先順位
- axis normalize
- duplicate binding候補
- Macroを含む静的出力候補と、割当に依存しない物理状態の投影
- config validation
- layout validation
- control protocol

### 25.2 Golden Test

- 正規化済みProfile JSON
- エラーコードと診断メッセージ
- Cairo headless image surfaceでの描画PNG
- 左右レイアウト
- compact/normal/detailed表示
- 日本語と英語
- 1.0/1.25/2.0 scale

画像goldenはフォント差を避けるため、CI専用フォント環境を固定するか、図形領域とtext layoutを分離して比較する。

### 25.3 Fuzz Test

最低限:

```text
FuzzNormalizeOuterText
FuzzDecodeBase64
FuzzDecodeLZMAEnvelope
FuzzDecodeMsgpackString
FuzzParseRoot
FuzzNormalizeProfile
FuzzLoadConfig
FuzzLoadLayout
```

期待条件:

- panicなし
- 設定上限を超えるメモリ確保なし
- timeout内終了
- 部分データ永続化なし

### 25.4 Integration Test

- synthetic sysfs/hidraw metadataとOS pipeを使う。
- type57の30 source ID press/release、malformed、padding非解釈
- 初期unknown、counter loss疑い、切断とgeneration更新
- selected identityのrenumber/reconnect、no event open、fd cleanup
- opt-in production reader実機操作はsyntheticとは別に記録し、未実施をpassにしない。
- permission denied
- config atomic rename
- source file partial write
- control socket concurrency

### 25.5 Wayland Integration Test

ネスト可能なCompositorまたはheadless backendで以下を検証する。

- Layer Shell surface作成
- overlay layer
- anchors/margins
- monitor selection
- click-through
- show/hideによるmap/unmap
- fractional scaling

CIで再現できないDirect Scanout、VRR、HDR、Protonフルスクリーンは実機試験とする。

### 25.6 実機マトリクス

| 環境 | 必須 |
|---|---:|
| Hyprland + NVIDIA | 必須 |
| Bodycam + Proton + borderless | 必須 |
| Bodycam + Proton + fullscreen | 必須 |
| 複数モニター、DP-2指定 | 必須 |
| 100% / fractional / 200% scale | 必須 |
| VRR on/off | 必須 |
| HDR on/off | 必須 |
| Gamescope nested | 推奨 |
| Sway | 推奨 |
| KDE Plasma Wayland | 推奨 |

Niriはv1後の対象で、実機動作は未検証。v1の必須試験には含めない。

### 25.7 長時間試験

24時間連続実行し、以下を計測する。

- RSS増加
- goroutine数
- open fd数
- CPU idle
- reconnect回数
- dropped event数
- GTK warning

### 25.8 提供サンプルの扱い

ユーザー提供の実プロファイルを公開リポジトリへそのままcommitしない。解析済み構造を再現する最小のsynthetic fixtureを生成する。

必須fixture:

- str16 bundle形式
- str32 single-profile形式
- legacy numeric code
- modern symbolic code
- labelあり/なし
- analog WASD
- macro/long/double
- corrupted LZMA
- declared length mismatch

破損した2つ目の提供サンプルと同等のfixtureでは、`ERR_IMPORT_LZMA_CORRUPT`となり、部分JSONを採用しないことを確認する。

---

## 26. CI・品質管理

### 26.1 Pull Request CI

- `gofmt`差分なし
- `go vet ./...`
- `staticcheck ./...`
- `scripts/test-wayland.sh go test -race -shuffle=on -count=1 ./...`（native childを含む）
- short fuzz smoke
- Arch native build container（Ubuntu distribution対応はv1後）
- CGo/GTK/layer-shell compile smoke
- license check
- vulnerability scan
- generated files差分確認

### 26.2 Release CI

- clean tag build
- version consistency
- full tests
- package smoke test
- checksums
- SBOM
- signatures
- release notes生成
- binary起動確認

### 26.3 依存固定

- Go toolchain patch versionをCIで固定。
- gotk4は互換性確認済みcommit/pseudo-versionへ固定。
- gtk4-layer-shellのサポート最小版を明記。
- Dependabot/Renovateの自動更新は、CGo/GTK依存について自動mergeしない。

---

## 27. ロギングと観測性

### 27.1 ログ

既定はstderrへstructured textを出し、systemd利用時はjournaldへ流す。独自ログファイルは既定で作らない。

レベル:

```text
error
warn
info
debug
trace
```

既定`info`。

### 27.2 メトリクス

ネットワークexporterは設けない。`azerlay status --json`で次を取得できる。

- uptime
- visible
- active profile/source
- active device identity（既存control schemaのevent_nodesは未接続の空配列を維持）
- event rate
- detected invalidation count（未検出のreport欠落を数えられるとは主張しない）
- render rate
- last reload result
- current generation
- degraded reasons

### 27.3 個人情報抑制

既定ログへ次を出さない。

- 全入力イベント列
- マクロ内容
- ユーザーラベル全文
- エクスポート原文
- ローカルAzeron DB内容

---

## 28. 受入条件

Azerlay 1.0は、以下をすべて満たした時点で完成とする。

### AC-001 インポート

正常な提供サンプル相当データを、`Base64URL → LZMA-Alone → MessagePack str16 → JSON`として厳密に読込める。プロファイル名、41 inputs、ラベル、WASDスティック等を復元できる。最初の`{`検索などのヒューリスティックを使用しない。

### AC-002 破損拒否

破損したstr32サンプル相当データを`ERR_IMPORT_LZMA_CORRUPT`として拒否し、部分プロファイルを保存しない。

### AC-003 形式世代

v1は、[binding conversion decision](decisions/binding-conversion.md)に記載した根拠のあるSoftware 2.0.2の閉じた変換行だけをCanonical Bindingへ変換し、行に一致しない値は`UnknownBinding`として保持する。現行のTurbo/Macro/角度は設定値の保持のみで、入力実行や90度の座標変換を意味しない。legacy numeric valueの変換はv1対象外とし、v1完了後の[Issue #102](https://github.com/sh4869221b/azerlay/issues/102)で扱う。未対応世代は受け入れない。

### AC-004 ローカル設定

対応版Azeron Softwareのローカル設定を自動検出または明示パスで読取り、設定変更を安全に反映できる。対応外版では誤読せず、理由とimport手順を提示する。

### AC-005 デバイス

Cyborg IIの確認済みinterface04だけを検出し、他機器・evdevを監視しない。再接続はidentityを維持しunknownから新通知を観測する。公式Software起動前提で実機確認し、2秒目標の測定と状態完全保証は別事項とする。

### AC-006 リアルタイム表示

v1ではHyprland上で、Bodycamのフルスクリーンまたはborderless表示より上にオーバーレイを表示し、入力p95 20ms以下でハイライトする。Niriはv1後の対象で、動作未検証とする。

### AC-007 透過

オーバーレイ領域上のマウスクリック、スクロール、移動がゲームへ到達し、Azerlayがkeyboard focusを取得しない。

### AC-008 入力非干渉

Azerlay起動前後でBodycamのAzeron入力が欠落・二重化・遅延増大しない。grab、inject、remapを行わない。

### AC-009 曖昧性

静的な割当候補の表示では、同じ出力へ複数ボタンが割り当てられている場合に全候補を示し、単一候補を断定しない。物理通知によるハイライトは通知されたsource IDに対応する1つのcontrolだけを更新し、出力の候補群へ伝播させない。

### AC-010 アナログ

Thumbstickの方向・入力量とWASD/アナログモードのlive表示は延期する。type57による物理ボタン通知ではアナログ入力を解釈せず、対応を主張しない。

### AC-011 設定復旧

設定・プロファイルの更新が不正でもlast-known-good表示を維持し、修正後に自動復帰する。

### AC-012 権限

root起動と`input`グループ追加を必要とせず、対象Azeron限定udev ruleで動作する。

### AC-013 安全性

fuzz testでpanic、無制限メモリ確保、部分保存が発生しない。socketは同一ユーザーだけが操作できる。

### AC-014 オフライン

通常実行時の外部ネットワーク通信が0であり、テレメトリーを送らない。

### AC-015 診断

主要な失敗条件を`azerlay doctor`が識別し、修正可能な手順を返す。

### AC-016 配布

クリーンな対応環境で、公開tarballまたはArch packageから導入し、READMEだけで起動・権限設定・インポート・自動起動まで完了できる。

---

## 29. 要調査事項

### 29.1 リリース阻害一覧

| ID | 調査内容 | 目的 | 阻害範囲 |
|---|---|---|---|
| R-001 | Azeron `types`値、legacy `keyValues/metaValues`、modern symbolic nameの完全対応表 | Canonical Bindingへ正確に変換 | インポート全体 |
| R-002 | Cyborg II左手用の`input ID`/pin→物理位置対応と改版差（v1）。右手用はv1後の対応へ延期 | 正しい模式図ハイライト | Cyborg II左手用のv1正式対応。右手用はv1を阻害しない |
| R-003 | Linux版Azeron Software 2.xのローカル保存パス、形式、locking | LocalSource実装 | ローカル自動読取 |
| R-004 | active profile、favorite、software/on-board状態の保存方法 | 自動選択 | プロファイル自動追従 |
| R-005 | Cyborg II各改版のVID/PID、USB interface、hidraw descriptor（[観測済み署名の決定](decisions/device-identity.md)あり、他の改版・左右・動作モードは未検証） | udevとDeviceGroup | 未観測範囲の自動検出・権限、配布時のACL検証 |
| R-006 | gotk4 branch 4と対象GTK4版、native GtkWindow/GdkSurface pointer連携 | 安定したCGo bridge | Overlay起動 |
| R-007 | [Hyprland v1配置の実測と決定](decisions/compositor-placement.md)：anchor、exclusive zone、monitor指定。Niriはv1後、動作未検証 | 正しい配置 | Overlay正式対応 |
| R-008 | GDK empty input regionのmap/remap/hotplug後挙動（[アプリ起点のremap実測](decisions/overlay-abi.md)あり。compositor起点のremapと物理hotplugは未検証） | 完全click-through | Overlay正式対応 |
| R-009 | [connector選択、fractional scale、一時出力の消失・再作成](decisions/compositor-placement.md)。同じキーへの復帰は未検証、物理hotplugも未検証 | 複数モニター | Multi-monitor正式対応 |
| R-010 | Direct Scanout、VRR、HDR、Gamescopeへの影響 | 性能・ゲーム互換性 | 性能保証 |
| R-011 | Bodycam現行バージョンの全キーバインドとアクション名 | built-in game profile | Bodycamプロファイル |
| R-012 | プロジェクト名、GitHub名、商標、依存ライセンス | 公開上の安全 | 公開リリース |
| R-013 | Azeron exportのSoftware 2.x各版、bundle/single、str8/fixstrの実在例 | parser互換性 | インポート互換表 |
| R-014 | LevelDB/IndexedDB使用時の稼働中安全読取 | DB破損回避 | LocalSource |

### 29.2 非阻害だが調査する事項

| ID | 調査内容 | フォールバック |
|---|---|---|
| R-020 | 旧evdevの`MSC_SCAN`調査は終了し、確認済みtype57 source IDによる物理識別へ置換 | evdevへのfallbackなし。静的割当候補は物理通知と分離 |
| R-021 | type57の30物理source IDは確認済み。last-observed契約を採用し、未検出末尾欠落はstaleになり得る | onboard profile/analog/連続現在状態保証は対象外 |
| R-022 | Sway/KDE等の追加Compositor | 準対応扱い |
| R-023 | AppImage/Flatpakでhidraw、udev、Layer Shellを安全に配布可能か | tarball/Native package |
| R-024 | ゲームプロセス検出による自動プロファイル切替 | 手動選択 |

### 29.3 調査時に収集する資料

- Azeron Software版番号。
- Cyborg II firmware版。
- 左右モデルのexport。
- Software profile / on-board profileのexport。
- single/long/double/macro/turbo/analog全種を含む匿名化fixture。
- `lsusb -v`、`udevadm info`、`evtest`能力情報。
- `/proc/bus/input/devices`。
- v1対象Hyprlandのversion。Niri/Gamescopeはv1後に検証する場合のversion。
- fractional scaling、VRR、HDR別の挙動。

秘密や個人ラベルを除去し、公開fixtureはsynthetic化する。

---

## 30. 主要な設計判断

| 判断 | 採用 | 理由 |
|---|---|---|
| 言語 | Go | 入力・設定監視・CLI・単一バイナリ構成が簡潔。性能上十分。 |
| GUI | gotk4 / GTK4 | WaylandとLayer Shell統合が堅実。 |
| Layer Shell | gtk4-layer-shell + 小型CGo | 現行GTK4対応を直接利用し、古いGTK3用Go wrapperを避ける。 |
| 入力 | 限定hidraw | 確認済み物理source通知をread-onlyで扱える。 |
| 描画 | Cairo | 小さな2D模式図にwgpu等は不要。 |
| import parser | 厳密pipeline | `Base64URL→LZMA→MessagePack String→JSON`を実データで確認。ヒューリスティックを排除。 |
| プロファイル | RawとNormalizedを分離 | Azeron形式更新をadapterへ閉じ込める。 |
| UI制御 | CLI + TOML + Unix socket | Web UIや別デーモンなしで運用可能。 |
| 権限 | udev uaccess | root・input groupを避ける。 |
| 入力同定 | type57 source ID | 出力割当に依存せず30物理controlの最終観測状態を投影する。 |
| 配布 | Native package/tarball | GTK/Layer Shell/udevとの統合を明確にする。 |
| ネットワーク | なし | プライバシー、単純性、ゲーム中の安定性。 |

---

## 31. 実装時に変更してはならない中核原則

1. Azeronエクスポートを最初の`{`検索で復元しない。
2. 破損圧縮データの部分JSONを採用しない。
3. Raw Azeron schemaを描画層へ漏らさない。
4. 一般キーボード全体を監視しない。
5. root、`input`グループ、grab、uinputを安易に要求しない。
6. 重複割当の物理元を推測で断定しない。
7. GTKオブジェクトを任意goroutineから操作しない。
8. draw callbackでI/O・parse・待機を行わない。
9. 設定再読込でlast-known-goodを破壊しない。
10. CGoをLayer Shell境界の外へ拡散しない。
11. 不要なWeb技術、DB、サービス分割、抽象化層を追加しない。
12. Azeron設定へ書き込まない。
13. 通常利用でネットワークへ接続しない。

---

## 32. 参考資料

技術選定と互換性確認時に参照する一次資料・公式資料。

- Go 1.27 Release Notes: https://go.dev/doc/go1.27
- Go Downloads / Release history: https://go.dev/dl/
- Linux input subsystem userspace API: https://docs.kernel.org/input/input_uapi.html
- Linux event interface: https://docs.kernel.org/input/event-codes.html
- GTK4 DrawingArea: https://docs.gtk.org/gtk4/class.DrawingArea.html
- GDK Surface input region: https://docs.gtk.org/gdk4/method.Surface.set_input_region.html
- gtk4-layer-shell: https://github.com/wmww/gtk4-layer-shell
- gotk4: https://github.com/diamondburned/gotk4
- Linux hidraw: https://docs.kernel.org/hid/hidraw.html
- xz/lzma for Go: https://pkg.go.dev/github.com/ulikunitz/xz/lzma
- fswatcher: https://github.com/fswatcher/fswatcher
- go-toml/v2: https://github.com/pelletier/go-toml
- Azeron Software / manuals: https://www.azeron.eu/

Azeronエクスポート内部形式、ローカルSoftware store、Cyborg IIの物理input ID対応は公開仕様として確定していないため、実機・現行版Software・匿名化fixtureによる検証を優先する。

---

## 33. 最終結論

Azerlay 1.0の中核構成は次で固定する。

```text
Go 1.27.x
+ qualified hidraw passive reader
+ gotk4 / GTK4
+ gtk4-layer-shell（小型CGo bridge）
+ Cairo
+ xz/lzma
+ encoding/json
+ fswatcher
+ go-toml/v2
+ udev uaccess
```

入力プロファイルは、厳密に検証したAzeron exportと、要調査のAzeron Softwareローカル設定から取得する。UIは単一のWayland overlay、操作はCLI・TOML・Unix socketで完結させる。入力を奪わず、書き換えず、注入せず、Azeronの既存設定とゲーム操作を妨げないことを最上位要件とする。

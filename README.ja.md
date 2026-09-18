# Decio

コンテキストを型付きの判断に変換し、必要に応じて事前定義された action を dispatch します。

[English](README.md)

```text
context
   │
   ▼
 Decio
   │
   ├── choice
   ├── boolean
   └── score
   │
   ▼
pre-declared action
```

```sh
git diff | decio \
  --boolean "Does this change require running tests?" \
  --result-exit-code
```

![Decio のデモ: git diff を decio にパイプすると true を返し、事前定義されたテストコマンドを実行します](docs/demo.gif)

## Decio とは

Decio は、コンテキストを型付きの判断（typed decision）に変換し、必要に応じて事前定義された action を dispatch する汎用的な判断 CLI です。Unix ワークフロー、hook、CI/CD、エージェント、自動化向けに設計されています。Claude Code Hooks は代表的な利用例の 1 つですが、必須ではありません。Decio 自体は Claude 固有の前提を持ちません。

Decio は任意のコマンドを生成または実行しません。action として実行できるすべてのコマンドは、信頼できる設定ファイルで宣言する必要があります。判断結果も自由文のモデル出力もシェル断片として扱いません。境界は次のとおりです。

```text
context → decision → pre-declared action
```

## 基本概念

Decio の中核モデルは次のとおりです。

```text
Input → Decision → Action
```

- **入力** は、判断に必要なコンテキストを取得します。
- **判断** は、型付きの結果、`choice`、`boolean`、または `score` を返します。
- **Action** は、設定されている場合に、その判断に関連付けられた事前定義された操作を実行します。

Decio は判断を生成した時点で実行を完了できます。dispatch は任意です。そのため、`コンテキスト → 型付きの判断` は、Decio を自由文生成 CLI にすることなく、シェル条件、CI/CD ゲート、後続の自動化だけでも利用できます。

## インストール

リリース済みバイナリを Homebrew でインストールします。

```sh
brew install youyo/tap/decio
```

または Go でインストールします。

```sh
go install github.com/youyo/decio@latest
```

判断を行う前に Jev API キーを設定します。

```sh
export TYPESAFE_API_KEY="..."
```

### シェル補完

`decio completion <shell>` で zsh、bash、fish、PowerShell 向けの補完スクリプトを生成できます。zsh では起動のたびに eval するのではなく、`fpath` 上のディレクトリにスクリプトを置き、補完キャッシュを作り直す方法をおすすめします。

```sh
mkdir -p ~/.zsh/completions
decio completion zsh > ~/.zsh/completions/_decio
rm -f ~/.zcompdump*
exec zsh
```

`~/.zshrc` では `fpath=(~/.zsh/completions $fpath)` を `compinit` より前に置いてください。`eval "$(decio completion zsh)"` でも動きますが、`compinit` の実行後に置く必要があります。

## クイックスタート

Decio には 2 つの利用方法があります。単純なケースでは設定ファイルは不要です。判断の種類とプロンプトを CLI フラグで渡し、stdin を通してコンテキストをパイプします。ワークフローに複雑な入力や action が必要な場合は、宣言的な YAML 設定を使用します。

### シンプル: CLI フラグと stdin

この boolean の判断は Unix パイプラインで直接使用できます。

```sh
git diff | decio --boolean "Does this change require running tests?" --result-exit-code
```

Choice:

```sh
git diff | decio \
  --choice none --choice normal --choice security \
  --prompt "Determine the appropriate review level."
```

![Decio の choice デモ: 認証まわりの変更が security に分類され、事前定義されたセキュリティレビューが実行されます](docs/demo-choice.gif)

Score:

```sh
git diff | decio \
  --score "Rate the security risk of this change." \
  --min 0 --max 100 --json
```

![Decio の score デモ: 同じ変更が 100 点中 89 点と評価され、スコア範囲の action が実行されます](docs/demo-score.gif)

デフォルトの出力は単一の値です。後続のコードが provider、model、confidence などのメタデータを必要とする場合は `--json` を使用します。`--result-exit-code` を指定すると、action が実行されなかった場合の有効な `false` の boolean 結果は exit code `1` を返します。これにより型付きの判断をシェル条件として使用できます。provider およびその他の Decio の失敗では、引き続き定義済みのエラーコードを使用します。

### 宣言的な設定

コメント付きの設定テンプレートを開始点として生成し、編集してから検証します。

```sh
decio init --type boolean -o .decio.yaml
decio config validate -c .decio.yaml
```

設定したワークフローは次のように実行します。

```sh
decio -c .decio.yaml
```

## 設定

YAML 設定を読み込むには `-c`/`--config` を使用します。フラグを省略すると、Decio は現在のディレクトリの `.decio.yaml`、次に `.decio.yml` を探します。`decio config validate -c path/to/config.yaml` は provider を呼び出さずにファイルを検証します。

`decio init` は、コメント付きの設定テンプレートを標準出力または `-o`/`--output` で指定したパスに書き込みます。`--type` で `choice`、`boolean`、または `score` を選択し、既存ファイルを上書きするには `--force` を使用します。

スキーマは意図的に厳格です。不明なキーは拒否されます。

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `version` | integer | 設定のバージョンです。現在のバージョンは `1` です。 |
| `provider.type` | string | provider の名前です。現在は `jev` です。 |
| `provider.model` | string | provider のモデルです。Jev のデフォルトは `jev-latest` です。 |
| `input.sources.<name>` | object | 名前付きの入力元です。入力元の種類はちょうど 1 つ必要です。 |
| `input.sources.<name>.stdin` | boolean | 元のプロセスの stdin を使用します。 |
| `input.sources.<name>.command` | string | シェルコマンドを実行し、stdout を値として使用します。 |
| `input.sources.<name>.file` | string | ファイルを値として読み込みます。 |
| `input.sources.<name>.literal` | string | リテラル値を使用します。 |
| `input.sources.<name>.timeout` | duration | `5s` など、コマンド入力元のタイムアウトです。 |
| `decision.type` | `choice \| boolean \| score` | 作成する型付きの判断です。 |
| `decision.prompt` | string | provider に送る指示です。 |
| `decision.choices.<id>.description` | string | choice ID の説明です。choice の判断には少なくとも 2 つの ID が必要です。 |
| `decision.range.min`, `max` | integer | 包含的な score の範囲です。`min` は `max` より小さくする必要があります。 |
| `decision.levels` | list of strings | score の判断に使う、順序付きの定性的なレベルです。 |
| `actions` | map or list | `choice`/`boolean` では map、`score` では範囲の list です。 |
| `actions.<key>.command` | string | map action が選択されたときに実行するコマンドです。 |
| `actions[].min`, `max` | integer | 包含的な score-action の範囲です。範囲は重複できません。 |
| `actions.*.stdin` | `original \| none` | action の stdin モードです。デフォルトは `none` です。 |
| `actions.*.env` | map of strings | 追加の action 用環境変数です。 |

score の action では、`max` を省略して上限のない範囲を作成できます。action stdin の `original` は元のプロセスの stdin を意味し、正規化された複数入力元の provider 入力ではありません。

完全な例は [`examples/`](examples/) を参照してください。

## 利用例

### Claude Code Hooks

[`examples/claude-code/post-tool.yaml`](examples/claude-code/post-tool.yaml) の設定は、stdin で hook イベントを受け取り、現在の diff を収集し、元の hook イベントを stdin にした後続コマンドを実行できます。

対応する `settings.json` の項目では、command hook として呼び出せます。

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash|Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "decio -c /absolute/path/to/examples/claude-code/post-tool.yaml"
          }
        ]
      }
    ]
  }
}
```

絶対パスの設定ファイルを使用し、hook プロセスが `TYPESAFE_API_KEY` にアクセスできるようにしてください。action のコマンドは設定で宣言され、provider の出力から生成されることはありません。

### Git Hooks

Git hook はステージ済みの diff から型付きの判断を作成できます。`--result-exit-code` を指定すると、`0` は有効な `true` の結果、`1` は action が実行されなかった場合の有効な `false` の結果を意味します。`2` 以上のコードは Decio または provider の失敗を示します。

```sh
git diff --cached | decio \
  --boolean "Does this staged change require running tests?" \
  --result-exit-code
```

### CI/CD

CI は JSON メタデータを利用し、標準的なシェルツールで後続のゲートを作成できます。

```sh
git diff HEAD~1 | decio \
  --score "Rate the security risk of this change." \
  --min 0 --max 100 --json | jq -e '.value < 60'
```

GitHub Actions はリリース済みバイナリをインストールし、後続のステップで使用できます。

```yaml
- uses: youyo/decio@v0
  with:
    version: latest        # or v0.1.1
- run: git diff HEAD~1 | decio --boolean "Does this change require running tests?" --result-exit-code
  env:
    TYPESAFE_API_KEY: ${{ secrets.TYPESAFE_API_KEY }}
```

ワークフローが `git diff HEAD~1` を必要とする場合は、`actions/checkout` に `fetch-depth: 2` を設定します。

再利用可能な CI ポリシーでは、入力と action のポリシーを設定ファイルに置き、`decio -c .decio.yaml --json` を実行します。

### シェル自動化

単純な結果はシェル変数に便利です。`--json` は `jq` などのツールに安定したメタデータを公開します。

```sh
value=$(git diff | decio --boolean "Should tests run?")
printf 'decision=%s\n' "$value"
```

action は任意かつ事前定義されているため、パイプラインは型付きの結果を自分で処理するか、設定済みの action を dispatch するかを選択できます。

## 入力元

入力は Decio の外部から stdin を通して受け取ることも、Decio が設定された入力元から収集することもできます。これにより、通常の Unix パイプラインと再利用可能な宣言的ワークフローの両方を簡潔に扱えます。

外部コンテキストは、フラグによる実行に直接パイプできます。

```sh
git diff | decio --boolean "Does this change require running tests?"
```

Decio は設定されたコマンドからコンテキスト自体を取得することもできます。

```yaml
input:
  sources:
    diff:
      command: git diff HEAD~1
```

各入力元の種類には明確な役割があります。

- `stdin`: 呼び出し元から渡されるコンテキストです。hook イベントやパイプされた diff などです。
- `command`: 事前定義されたコマンドを実行して Decio 自身が収集するコンテキストです。
- `file`: ディスクから読み込むポリシーまたは参照ドキュメントです。
- `literal`: リポジトリ名や環境名などの固定値です。

名前付きの入力元が設定されていない場合、パイプされた stdin が 1 つのテキスト入力になります。名前付きの入力元は YAML 設定の順序で収集され、構造化された状態として provider に送られます。

```yaml
input:
  sources:
    event:
      stdin: true
    diff:
      command: git diff
    policy:
      file: .decio/security-policy.md
    repository:
      literal: backend-api
```

コマンド入力元は `sh -c` を通して実行され、stdout を取得します。0 以外の終了またはタイムアウトは入力エラーとして扱います。stderr は診断情報用に保持され、provider 入力に暗黙的に追加されることはありません。元の stdin は一度だけ読み込まれるため、判断と、要求された場合の dispatch された action の両方で使用できます。

## Dispatch と action の環境

Dispatch は任意です。choice と boolean の判断では、選択された値が action key（boolean では `true` または `false`）として使用されます。score では、結果を含む最初の包含範囲が選択されます。action がない場合は出力のみの実行となり、エラーではありません。

action は設定ですでに宣言されたコマンドから選択されます。Decio はモデルの応答をコマンドに埋め込んだり、provider が記述したコマンドを実行したりしません。

各 action は次のメタデータ変数を受け取ります。

```text
DECIO_TYPE
DECIO_VALUE
DECIO_PROVIDER
DECIO_MODEL
DECIO_CONFIDENCE
```

カスタム `env` の値は次の置換だけに対応します。

```text
{{ result.value }}
{{ result.type }}
{{ result.confidence }}
{{ result.provider }}
{{ result.model }}
```

provider が confidence を返さなかった場合、confidence は空です。action の stdout と stderr はどちらも Decio の stderr に書き込まれます。Decio の stdout は結果用に予約されています。action の exit code はそのまま返します。

## 出力

プレーン出力には装飾がありません。

- choice: 選択された choice ID
- boolean: `true` または `false`
- score: 整数の score

`--json` を指定すると、Decio は安定した結果形式を出力します。

```json
{
  "type": "choice",
  "value": "security",
  "confidence": 0.94,
  "provider": "jev",
  "model": "jev-latest",
  "action": {
    "executed": true,
    "exit_code": 0
  }
}
```

利用できない場合、`confidence` と `action` は省略されます。

## Exit code

| コード | 意味 |
| ---: | --- |
| `0` | `--result-exit-code` が有効でない限り、`false` の boolean 結果を含めて Decio は成功します。 |
| `1` | `--result-exit-code` を指定し、action が実行されなかった場合の有効な `false` の boolean 結果です。 |
| `2` | 使い方または設定のエラーです。 |
| `3` | 入力収集の失敗です。 |
| `4` | provider の失敗です。 |
| `5` | provider の結果が無効です。 |
| `6` | action の dispatch 準備の失敗です。 |
| `N` | dispatch された action が返した exit code です。 |

## Jev provider

Decio は TypeSafe の Jev 評価エンドポイントにリクエストを送ります。これらはコミット済みの YAML ファイルではなく、環境変数で設定してください。

```sh
export TYPESAFE_API_KEY="..."
export TYPESAFE_BASE_URL="https://api.typesafe.ai"
```

`TYPESAFE_BASE_URL` のデフォルトは `https://api.typesafe.ai` です。クライアントは `/v1/systemone` に POST します。設定されたモデルのデフォルトは `jev-latest` です。HTTP 429 と 529 の応答は、`Retry-After` が指定されていればそれを使用し、指定されていなければ 0.5s、1s、2s の待機時間で再試行します。CLI のタイムアウトはリクエストコンテキストに適用されます。

boolean の判断では Jev の `noul` 確率を使用します。値が `0.5` 以上なら `true`、`0.5` 未満なら `false` です。

score の判断では Jev の順序付き score レベルを使用します。Jev score が `s`、レベルが `n` 個の場合、Decio は次の式で設定された整数の包含範囲に変換します。

```text
round(min + (s / (n - 1)) * (max - min))
```

デフォルトのレベルは `very low`、`low`、`moderate`、`high`、`very high` の 5 つの順序付きラベルです。`decision.levels` を設定すると置き換えられます。provider の契約により、少なくとも 2 つのレベルが必要です。choice と score の判断は、返された Jev の confidence も保持します。

## AI エージェント向け

埋め込みリファレンス全体は `decio docs` で読めます。トピック一覧は
`decio docs --list`、Claude Code Hook のレシピは `decio docs claude-code`
で確認できます。

## 開発

リポジトリは mise で開発ツールを固定しています。

```sh
mise install
mise run build
mise run test
mise run lint
mise run check
```

`mise run check` は Go ファイルをフォーマットし、リンターを実行し、テストスイートを実行します。`.goreleaser.yaml` と `.github/workflows/release.yaml` に記載された GoReleaser と GitHub の認証情報を設定した後、`mise run release` でリリースを作成します。

## セキュリティモデル

設定ファイルは実行可能なポリシーであり、シェルスクリプトと同じように扱う必要があります。Decio は provider が生成したコマンドテキストを実行しません。provider はユーザーがあらかじめ宣言したコマンドの中から選択するだけです。choice の値は識別子であり、シェル断片ではありません。任意のモデル出力がコマンド文字列に埋め込まれることはありません。

シークレットは環境またはシークレットストアに保持し、明示的に収集しない限り provider 入力に含めないでください。入力元の stderr は診断専用です。外部コマンドと HTTP 呼び出しには時間制限があり、不正な、または範囲外の判断は dispatch 前に拒否されます。

## ライセンス

MIT です。[`LICENSE`](LICENSE) を参照してください。

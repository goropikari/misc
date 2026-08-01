# DW Phase Workflow State

- **Global Step**: 1
- **Current Path**: - **Current Name**:
- **Current Name**: - **Phase Type**:
- **Phase Type**: - **Local Step**: 0
- **Local Step**: 0
- **Local Stage**: - **Phase Final Step**: no
- **Phase Final Step**: no
- **Step Name**: 1. フェーズ設計
- **Target**: .dev-workflow-phase/01_phase_design.md
- **Status**: REVIEWED

## 説明
トップレベル phase の責務、依存方向、完了条件、実装順、Phase Type を定義する。

## 進め方
- `.dev-workflow-phase/00_project_requirements.md` を読み、トップレベル phase を設計する。
- 必須メタデータ `- **Phases**: N` を書く。N は 1 以上 20 以下の整数。
- 各 `## Phase N: 名前` セクションに `- **Phase Type**: feature|layer` を書く。

## 進捗サマリ / Progress Summary
<!-- AI が現在の作業状況を随時記録し、完了したら $dw-phase review (Claude Code: /dw-phase review) を実行してください -->
- [ ] ターゲット範囲の作成・初期定義
- [ ] 詳細な内容の実装・調整
- [ ] 動作確認 / 自己テスト合格

## AI Agent Constraint / 制約事項
次へ進む指示（`$dw-phase next` コマンド、Claude Code では `/dw-phase next` の実行）があるまで、絶対にこれ以降のステップのファイルを生成・変更してはいけない。
Do NOT create or modify files for subsequent steps until explicitly instructed to proceed via `$dw-phase next` or `/dw-phase next`.

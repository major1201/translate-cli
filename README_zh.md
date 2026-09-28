# translate-cli

一个快速、离线优先的命令行翻译工具。

`translate-cli`（命令名 `tx`）在终端里直接翻译文本。它优先查询内置的
离线词典（即时、无网络、无需 API Key），词典覆盖不了的语言方向再回退到
流式的 LLM 翻译。

## 特性

- **自动识别源语言** —— 根据文本的 Unicode 文字系统推断源语言，通常无需
  手动指定 `-s`。可识别中文、英文、日文、韩文、俄文、阿拉伯文、泰文。
- **离线词典** —— 340 万词条的英→中词典（ECDICT）和约 12 万词条的中→英
  词典（CC-CEDICT），基于二分查找，启动仅需数毫秒。
- **LLM 翻译** —— 兼容任意 OpenAI Chat Completions API，逐 token 流式输出。
- **合理的默认方向** —— 中文输入译为英文，其他语言译为中文。

## 环境要求

- Go 1.26+
- GNU Make
- [7-Zip](https://www.7-zip.org/)（`7z`）—— 用于解压 ECDICT 数据
- `curl` 与 Python 3 —— 用于下载并转换 CC-CEDICT 数据

## 构建

```sh
git clone --recursive https://github.com/major1201/translate-cli.git
cd translate-cli
make build
```

首次构建时，`make` 会准备词典数据：

- 从 `ECDICT/stardict.7z` 解压出 `dict/stardict.csv`（约 232 MB）
- 下载 CC-CEDICT 并生成 `dict/cedict.csv`（约 8 MB）

这两个 CSV 均已加入 `.gitignore`，按需生成。最终生成的 `tx` 可执行文件
内置了词典，可以独立运行。

## 用法

```
tx [选项] <文本>
```

### 选项

| 选项 | 说明 |
| --- | --- |
| `-s <语言>` | 源语言（如 `en`、`zh`、`ja`）。省略时自动识别。 |
| `-t <语言>` | 目标语言。中文输入默认 `en`，其他语言默认 `zh`。 |
| `-d` | 仅查词典：不调用 LLM，无需 API Key。 |
| `-l` | 仅用 LLM：跳过本地词典。 |
| `-h` | 显示帮助。 |

`-d` 与 `-l` 互斥。

### 示例

```sh
$ tx apple
📖 apple  BrE /ˈæpl/, AmE /ˈæpəl/
   n. 苹果, 家伙

$ tx 德国
📖 德国  /dé guó/
   Germany

$ tx ドイツ                # 自动识别为日文 → 中文
🌐 LLM Translation (ja → zh):
德国

$ tx -d good              # 仅查离线词典
📖 good  BrE /gud/, AmE /ˈgʊd/
   n. 善行, 好处, 利益
   a. 好的, 优良的, 上等的, 愉快的, 有益的, 好心的, 慈善的, 虔诚的
   复数：goods
   比较级：better
   最高级：best

$ tx -l 德国              # 跳过词典，强制使用 LLM
🌐 LLM Translation (zh → en):
Germany

$ tx -t ja apple          # 显式指定目标语言
🌐 LLM Translation (en → ja):
りんご
```

## LLM 配置

LLM 翻译需要 API Key（`-d` 模式不需要）。配置从环境变量读取，`TX_LLM_*`
优先于 `OPENAI_*`：

| 变量 | 备选 | 默认值 |
| --- | --- | --- |
| `TX_LLM_API_KEY` | `OPENAI_API_KEY` | — |
| `TX_LLM_BASE_URL` | `OPENAI_BASE_URL` | `https://api.openai.com/v1` |
| `TX_LLM_MODEL` | `OPENAI_MODEL` | `gpt-4o-mini` |

`TX_LLM_BASE_URL` 可以指向任意支持流式输出的 OpenAI 兼容 Chat
Completions 端点：

```sh
export TX_LLM_API_KEY=sk-...
export TX_LLM_BASE_URL=https://your-provider.example.com/v1
export TX_LLM_MODEL=your-model
```

## 工作原理

1. 根据文本的 Unicode 文字系统识别源语言：假名 → `ja`，谚文 → `ko`，
   西里尔字母 → `ru`，阿拉伯字母 → `ar`，泰文 → `th`，汉字 → `zh`，
   拉丁字母/其他 → `en`。
2. 如果方向有本地词典覆盖（`en→zh` 或 `zh→en`）且未指定 `-l`，先查整句；
   多词短语再逐词尝试。
3. 命中则打印词典条目并结束。
4. 否则以流式方式调用 LLM 翻译并输出到 stdout。

## 词典数据

| 词典 | 方向 | 词条数 | 来源 | 许可证 |
| --- | --- | --- | --- | --- |
| [ECDICT](https://github.com/skywind3000/ECDICT) | 英 → 中 | 3,402,564 | git 子模块，构建时解压 | MIT |
| [CC-CEDICT](https://www.mdbg.net/chinese/export/cedict/) | 中 → 英 | 121,175 | 构建时下载并转换 | CC BY-SA 4.0 |

## 许可证

本项目采用 [MIT License](LICENSE)。

内置词典数据遵循各自的许可证（见上表）。

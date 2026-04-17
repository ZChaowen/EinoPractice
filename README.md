# 实验环境配置指南

## 实验环境

本项目是基于 CloudWeGo Eino 框架的大语言模型应用开发教学项目，包含多个实验模块，涵盖从基础聊天到高级 RAG 应用的完整学习路径。

### 开发环境

#### 1. 访问项目地址

- GitHub 仓库: [https://github.com/NuyoahCh/einotelos](https://github.com/NuyoahCh/einotelos)
- CloudWeGo Eino 官方文档: [https://rcn3ahrrdvjj.feishu.cn/wiki/space/7582137522705140933](https://rcn3ahrrdvjj.feishu.cn/wiki/space/7582137522705140933)

#### 2. 安装 Golang SDK

- **版本要求**: Go 1.18+（推荐使用 Go 1.24.10）
- **下载地址**: [https://go.dev/dl/](https://go.dev/dl/)
- **安装验证**:
  ```bash
  go version
  # 输出示例: go version go1.24.10 darwin/amd64
  ```

#### 3. 推荐使用 IDE

- **Visual Studio Code** (推荐)
  - 安装 Go 扩展: `Go` by Google
  - 支持代码高亮、自动补全、调试等功能
- **GoLand** by JetBrains
- 其他支持 Go 的代码编辑器

#### 4. 推荐使用 OS

- **Linux** (推荐)
- **macOS** 
- **Windows** (需安装 Git Bash 或 WSL)

如果没有设置 `$GOPATH`，可以使用默认路径：
```bash
cd ~/go/src/github.com/{your_username}/
```

#### 5. 安装项目依赖

```bash
# 下载并安装项目依赖包
go mod download
go mod tidy
```

#### 6. 配置环境变量

本项目需要配置 API Key 才能正常运行。支持多种 LLM 服务商：

##### DeepSeek（推荐入门使用）
```bash
export DEEPSEEK_API_KEY="your_deepseek_api_key"
```
- 注册地址: [https://platform.deepseek.com/](https://platform.deepseek.com/)
- 新用户赠送免费额度

**环境变量持久化（推荐）**

为避免每次重启终端都要重新设置，建议将环境变量添加到配置文件：

```bash
# macOS/Linux (使用 zsh)
echo 'export DEEPSEEK_API_KEY="your_deepseek_api_key"' >> ~/.zshrc
source ~/.zshrc

# macOS/Linux (使用 bash)
echo 'export DEEPSEEK_API_KEY="your_deepseek_api_key"' >> ~/.bashrc
source ~/.bashrc
```

## 实验步骤

### 1. 跟着设计文档已完成一遍

建议按照以下顺序学习各个实验模块：

1. **Lab 01 - 快速入门**: 理解基本的 ChatModel 使用方式
2. **Lab 02 - 工作流**: 学习如何编排复杂的对话流程
3. **Lab 03 - 生成配置**: 掌握不同的生成模式和错误处理
4. **Lab 04 - 提示词工程**: 学习高效的提示词设计
5. **Lab 05 - 文档处理**: 了解文档加载、解析、切分的完整流程
6. **Lab 06 - 向量化**: 掌握向量嵌入和相似度检索
7. **Lab 07 - Lambda**: 学习函数式编程在 LLM 中的应用
8. **Lab 08 - 索引器**: 构建高效的文档索引系统
9. **Lab 09 - RAG**: 实现完整的检索增强生成应用
10. **Lab 10 - 工具调用**: 让 LLM 能够调用外部工具

### 2. 运行实验代码

每个实验都可以独立运行。以 Lab 01 为例：

```bash
# 进入项目根目录
cd /path/to/einotelos

# 运行 Lab 01 快速入门
cd lab01
go run chat_quickstart.go
```

**常见运行方式：**

```bash
# 方式一：直接运行单个文件
go run lab01/chat_quickstart.go

# 方式二：进入目录后运行
cd lab01
go run chat_quickstart.go

# 方式三：构建后运行（推荐生产环境）
go build -o bin/chat_quickstart lab01/chat_quickstart.go
./bin/chat_quickstart
```

### 3. 修改参数进行调试

在运行过程中，可以：

- 修改提示词内容，观察输出变化
- 调整模型参数（temperature、max_tokens 等）
- 切换不同的模型进行对比测试
- 添加日志输出，跟踪程序执行流程

### 4. 最后自己完全手写实现一遍

为了深入理解框架原理：

- 不看原有代码，根据需求从零开始编写
- 实现过程中遇到问题，查阅官方文档
- 对比自己的实现与示例代码的差异
- 思考不同实现方式的优劣

### 5. 完成后查看 output 目录的文档

每个实验的 `output/` 目录下都有对应的 Markdown 文档：

- 包含实验的详细说明
- 代码的关键知识点解析
- 常见问题和解决方案
- 进阶学习建议

---

## 常见问题排查

### 依赖安装失败

```bash
# 清理缓存后重新安装
go clean -modcache
go mod download
```

## 进阶学习资源

- **CloudWeGo Eino 官方文档**: [https://rcn3ahrrdvjj.feishu.cn/wiki/space/7582137522705140933](https://rcn3ahrrdvjj.feishu.cn/wiki/space/7582137522705140933)
- **Go 语言官方文档**: [https://go.dev/doc/](https://go.dev/doc/)
- **LangChain 概念参考**: [https://python.langchain.com/](https://python.langchain.com/)

---

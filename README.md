# 项目介绍
用于学习 Go 的简单项目集合，包含一个命令行 RSS reader。

## 目录结构
- cmd/first_app 第一个 Go 程序示例
- cmd/message_copilot 简单输出示例
- cmd/rss_reader 简单 RSS reader（命令行）
- internal/opml OPML 文件处理包（导入导出）

## RSS Reader 使用方式

### 读取单个 RSS 源
```bash
go run ./cmd/rss_reader -url https://feeds.bbci.co.uk/news/rss.xml -limit 5
```

### 导入 OPML 文件并显示所有源
从 OPML 文件导入订阅源列表，并依次获取每个源的最新文章：

```bash
go run ./cmd/rss_reader -import-opml feeds.opml -limit 3
```

### 导出订阅源为 OPML 文件
将多个 RSS 源导出为 OPML 格式，便于在不同应用间共享：

```bash
go run ./cmd/rss_reader -export-opml feeds.opml -feeds "BBC News|https://feeds.bbci.co.uk/news/rss.xml|News,CNN|http://rss.cnn.com/rss/cnn_topstories.rss|News,TechCrunch|https://techcrunch.com/feed/|Technology"
```

**格式说明：** `-feeds` 参数使用管道符（|）分隔，格式为 `标题|URL|分类`（分类可选），多个源用逗号分隔。

### 命令行参数说明

| 参数 | 说明 | 示例 |
|-----|------|------|
| `-url` | RSS 源的 URL 地址 | `https://feeds.bbci.co.uk/news/rss.xml` |
| `-limit` | 显示的文章数量（默认 5） | `10` |
| `-timeout` | HTTP 请求超时时间（默认 10s） | `30s` |
| `-import-opml` | 导入 OPML 文件的路径 | `feeds.opml` |
| `-export-opml` | 导出 OPML 文件的保存路径 | `feeds.opml` |
| `-feeds` | 导出时的源列表（使用\|和,分隔） | 见上方导出示例 |

## OPML 文件格式

OPML（Outline Processor Markup Language）是一个标准格式，用于在应用程序之间交换订阅源列表。

### 示例 OPML 文件
```xml
<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head>
    <title>My RSS Feeds</title>
    <dateCreated>2026-05-30T18:00:00Z</dateCreated>
  </head>
  <body>
    <outline text="News" title="News">
      <outline text="BBC News" title="BBC News" type="rss" 
        xmlUrl="https://feeds.bbci.co.uk/news/rss.xml" 
        htmlUrl="https://www.bbc.com/news"/>
    </outline>
    <outline text="Technology" title="Technology">
      <outline text="TechCrunch" title="TechCrunch" type="rss" 
        xmlUrl="https://techcrunch.com/feed/" 
        htmlUrl="https://techcrunch.com"/>
    </outline>
  </body>
</opml>
```

## 功能特性

- ✅ 读取 RSS 源并显示最新文章
- ✅ 支持导入 OPML 格式的订阅源列表
- ✅ 支持导出订阅源为 OPML 文件
- ✅ 支持为源添加分类标签
- ✅ 可配置的超时时间和文章显示数量
- ✅ 完整的单元测试覆盖

## 相关链接
- [OPML 规范](http://www.opml.org/spec2.opml)
- [Go 项目布局](https://github.com/uiliugang/project-layout/blob/master/README_zh-CN.md)

## 个人学习计划
每个阶段的学习内容，会在 README.md 中进行更新，每天都会 commit 学习内容，欢迎大家一起学习交流。

- [x] 2026-02-25 了解 go 项目布局,并创建第一个 go 项目
- [x] 2026-05-30 实现 OPML 导入导出功能，支持订阅源分类和管理

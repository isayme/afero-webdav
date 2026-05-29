## 项目概述

- **模块**: `github.com/isayme/afero-webdav`
- **包名**: `webdavfs`
- **描述**: 基于 [afero](https://github.com/spf13/afero) 的 WebDAV 文件系统实现
- **底层客户端**: [gowebdav](https://github.com/studio-b12/gowebdav) v0.12+
- **参考实现**: afero-gdrive, afero-s3, afero-minio

## 已知约束

以下操作**不支持**（WebDAV 协议限制）：

| 方法 | 原因 |
|------|------|
| `O_RDWR` / `O_APPEND` / `O_EXCL` | WebDAV 基于 HTTP，不支持原子读写或追加 |
| `Chmod` | WebDAV 不暴露 Unix 权限位 |
| `Chtimes` | WebDAV 无独立修改时间的标准方法 |
| `Chown` | WebDAV 不暴露所有权元数据 |
| `Truncate` | WebDAV 不提供截断操作 |
| Write seek / `WriteAt` | WebDAV PUT 总是替换整个资源 |
| 写入时追加 (`O_APPEND`) | 同上 |

## 架构

```
Fs (fs.go) ─── *gowebdav.Client
  └── Create/Open → *File (file.go)
        ├── Read: HTTP GET + Range（按需流式读取，支持 seek）
        ├── Write: io.Pipe + PUT（后台 goroutine 流式上传）
        └── Readdir: PROPFIND depth 1

FileInfo (fileinfo.go) ─── os.FileInfo + ETag/ContentType
```

## 代码风格

- 注释使用英文，解释意图、约束、边界情况和取舍
- 避免 hard-coded values，常量和配置集中在定义处
- 测试文件按被测源文件拆分（`fs_test.go` / `file_test.go` / `fileinfo_test.go` / `errors_test.go`）
- 共享测试基础设施放在 `helpers_test.go`

## 工程惯例

- 行为变更前先制定计划并等待确认
- 优先小步提交、独立 review
- 默认运行 `go build ./... && go vet ./... && go test -count=1 ./...` 验证

# afero-webdav

An [afero](https://github.com/spf13/afero) filesystem implementation backed by a WebDAV server.

## Usage

```go
import (
    "github.com/isayme/afero-webdav"
    "github.com/studio-b12/gowebdav"
)

func main() {
    client := gowebdav.NewClient("https://webdav.example.com/dav/", "user", "password")
    fs := webdavfs.New(client)

    file, _ := fs.Create("/hello.txt")
    file.Write([]byte("Hello, WebDAV!"))
    file.Close()
}
```

## Features

- Create, read, write, and delete files
- Make and remove directories (with `MkdirAll`/`RemoveAll`)
- Rename/move files
- Directory listing (`Readdir`, `Readdirnames`)
- Range-based read with seek support (`Seek`, `ReadAt`)
- Streaming writes via `io.Pipe`

## Limitations

- `O_RDWR`, `O_APPEND`, and `O_EXCL` flags are not supported
- Write seeking is not supported
- `Chmod`, `Chtimes`, `Chown` are not supported
- `Truncate` is not supported

goftp
=====

Golang FTP library with Walk support.

## Features

* AUTH TLS support
* Walk 

## Sample
```go
package main

import (
    "crypto/sha256"
    "crypto/tls"
    "fmt"
    "io"
    "os"

    "encoding/hex"
    "gopkg.in/dutchcoders/goftp.v1"
)

func main() {
    var err error
    var ftp *goftp.FTP

    // For debug messages: goftp.ConnectDbg("ftp.server.com:21")
    if ftp, err = goftp.Connect("ftp.server.com:21"); err != nil {
        panic(err)
    }

    defer ftp.Close()
    fmt.Println("Successfully connected to", server)

    // TLS client authentication
    config := tls.Config{
        InsecureSkipVerify: true,
        ClientAuth:         tls.RequestClientCert,
    }

    if err = ftp.AuthTLS(config); err != nil {
        panic(err)
    }

    // Username / password authentication
    if err = ftp.Login("username", "password"); err != nil {
        panic(err)
    }

    if err = ftp.Cwd("/"); err != nil {
        panic(err)
    }

    var curpath string
    if curpath, err = ftp.Pwd(); err != nil {
        panic(err)
    }

    fmt.Printf("Current path: %s", curpath)

    // Get directory listing
    var files []string
    if files, err = ftp.List(""); err != nil {
        panic(err)
    }
    fmt.Println("Directory listing:", files)

    // Upload a file
    var file *os.File
    if file, err = os.Open("/tmp/test.txt"); err != nil {
        panic(err)
    }

    if err := ftp.Stor("/test.txt", file); err != nil {
        panic(err)
    }

    // Download each file into local memory, and calculate it's sha256 hash
    err = ftp.Walk("/", func(path string, info os.FileMode, err error) error {
        _, err = ftp.Retr(path, func(r io.Reader) error {
            var hasher = sha256.New()
            if _, err = io.Copy(hasher, r); err != nil {
                return err
            }

            hash := fmt.Sprintf("%s %x", path, hex.EncodeToString(hasher.Sum(nil)))
            fmt.Println(hash)

            return err
        })

        return nil
    })
}
````

## Contributions

Contributions are welcome.

* Sourav Datta: for his work on the anonymous user login and multiline return status.
* Vincenzo La Spesa: for his work on resolving login issues with specific ftp servers


## Creators

**Remco Verhoef**
- <https://twitter.com/remco_verhoef>
- <https://twitter.com/dutchcoders>

## Copyright and license

Code and documentation copyright 2011-2014 Remco Verhoef.
Code released under [the MIT license](LICENSE).


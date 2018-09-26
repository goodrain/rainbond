## Coquelicot

Coquelicot is an easy to use server-side upload service written in Go.

It is compatible with the [jQuery-File-Upload](https://github.com/blueimp/jQuery-File-Upload)
widget and supports chunked and resumable file upload. 

Using Coquelicot, you can add upload functionality to your project
very easily. Just download and unzip the Coquelicot binary distribution
for your OS and configure the jQuery-File-Upload widget.

![logo](http://go-tsunami.com/assets/images/coquelicotLogo.jpg)

### Server Setup

You can use a binary release or get the project if you have a working Go installation.

#### Binary Release

Grab the latest [binary release](https://github.com/gotsunami/coquelicot/releases) for you system. Unzip it
and run

```
$ ./coquelicot -storage /tmp/files -host localhost:9073
```

to store uploaded files into `/tmp/files` and make the application listen on the loopback interface port 9073
(run `coquelicot.exe` on Windows).

#### Source Release

Grab the latest stable version with:

```
$ go get gopkg.in/gotsunami/coquelicot.v1
```

See the [API documentation](http://gopkg.in/gotsunami/coquelicot.v1).

### jQuery-File-Upload Setup (Client)

The `fileupload` object needs the `xhrFields`, `maxChunkSize` and `add` fields to be defined.

- `xhrFields`: enables sending of cross-domain cookies, which is required to properly handle chunks of data server-side
- `maxChunkSize`: enables uploading chunks of file
- `add`: overwrites the default `add` handler to support resuming file upload

Download the [latest release](https://github.com/blueimp/jQuery-File-Upload/releases) of jQuery-File-Upload,
edit the `js/main.js` file in the distribution and make the `fileupload` initialization look like
(replacing the `localhost:9073` part with the name:port of your server running the `coquelicot` program):

```
$('#fileupload').fileupload({
    // Send cross-domain cookies
    xhrFields: {withCredentials: true},
    url: 'http://localhost:9073/files',
    // Chunk size in bytes
    maxChunkSize: 1000000,
    // Enable file resume
    add: function (e, data) {
        var that = this;
        $.ajax({
            url: 'http://localhost:9073/resume',
            xhrFields: {withCredentials: true},
            data: {file: data.files[0].name}
        }).done(function(result) {
            var file = result.file;
            data.uploadedBytes = file && file.size;
            $.blueimp.fileupload.prototype.options.add.call(that, e, data);
        });
    }
});
```

# Draft API

## Response

Single file:
```
{
    "directory": "/image/2014/6f/w015i",
    "type": "image",
    "versions": {
        "original": {
            "filename": "original-15h1.png",
            "height": 60,
            "mime": "image/png",
            "url": "/image/2014/6f/w015i/original-15h1.png",
            "size": 3464,
            "width": 53
        },
        "pic": {
            "filename": "pic-15h1.png",
            "height": 90,
            "mime": "image/png",
            "url": "/image/2014/6f/w015i/pic-15h1.png",
            "size": 7648,
            "width": 120
        }
    }
}
```

Multiple files:
```
{
    "files": [
        {...},
        {...}
    ]
}
```

## Binary upload

```
POST /files HTTP/1.1
Content-Length: 21744
Accept: application/json
Content-Disposition: attachment; filename="pic.jpg"

...bytes...
```

## Multipart

```
POST /files HTTP/1.1
Content-Length: 21929
Accept: application/json
Content-Type: multipart/form-data; boundary=----5XhQf4IXV9Q26uHM

------5XhQf4IXV9Q26uHM
Content-Disposition: form-data; name="files[]"; filename="pic.jpg"
Content-Type: image/jpeg

...bytes...
```

## Chunked multipart

First request create temporary file
```
POST /files HTTP/1.1
Content-Length: 25185
Content-Range: bytes 0-24999/52097
Content-Disposition: attachment; filename="kino.jpg"
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryAD3u12ABYZTJiIy3

------WebKitFormBoundaryAD3u12ABYZTJiIy3
Content-Disposition: form-data; name="files[]"; filename="kino.jpg"
Content-Type: image/jpeg

...bytes...
------WebKitFormBoundaryAD3u12ABYZTJiIy3--
```

Second request write chunk to exists temporary file
```
POST /files HTTP/1.1
Content-Length: 25185
Content-Range: bytes 25000-49999/52097
Content-Disposition: attachment; filename="kino.jpg"
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryvbE2anvAQyF3PWZS

------WebKitFormBoundaryvbE2anvAQyF3PWZS
Content-Disposition: form-data; name="files[]"; filename="kino.jpg"
Content-Type: image/jpeg

...bytes...
------WebKitFormBoundaryvbE2anvAQyF3PWZS--
```

Last request write chunk to exists temporary file, complete upload, create attachment.
```
POST /files HTTP/1.1
Content-Length: 2282
Content-Range: bytes 50000-52096/52097
Content-Disposition: attachment; filename="kino.jpg"
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryrHBafxSExXodxlnL

------WebKitFormBoundaryrHBafxSExXodxlnL
Content-Disposition: form-data; name="files[]"; filename="kino.jpg"
Content-Type: image/jpeg

...bytes...
------WebKitFormBoundaryrHBafxSExXodxlnL--
```

## Chunked binary

```
POST /files HTTP/1.1
Content-Length: 10240
Content-Range: bytes 0-10239/36431
Accept: application/json
Content-Disposition: attachment; filename="pic.jpg"
Content-Type: image/jpeg

...bytes...
```

## Check chunked upload progress

```
PUT /files/some_url HTTP/1.1
Content-Length: 0
Content-Range: bytes */2000000
```

```
HTTP/1.1 308 Resume Incomplete
Content-Length: 0
Range: 0-42
```

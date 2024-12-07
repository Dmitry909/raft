# Raft algorithm

From root run:
```
go run app/main.go 8000
```

HTTP-handlers:
1. `GET read?key=key1`.
    - 302, if this node is master
    - 404, if key doesn't exist.
    - 200, if exists.
2. `POST update?key=key1&value=value1`.
    - 404, if key doesn't exist.
    - 200, if key existed.
3. `DELETE delete?key=key1`
    - 404, if key didn't exist.
    - 200, if key existed.

```
curl -i -X PUT "http://localhost:8000/update?key=key1&value=value1.1"

curl -i -X PUT "http://localhost:8000/read?key=key1"

curl -i -X PUT "http://localhost:8000/delete?key=key1"
```

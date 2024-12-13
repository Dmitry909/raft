# Raft algorithm

From root run:
```
go run app/main.go 8000 "8000,8001,8002" 500
go run app/main.go 8001 "8000,8001,8002" 1000
go run app/main.go 8002 "8000,8001,8002" 1500
```

External (user's) HTTP-handlers:
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

Internal (node-to-node) HTTP-handlers:
1. `POST vote_request`. Body contains:
    - sender's term
    - log length
    - log's last term (or 0, if log is empty)
2. `POST vote_response`
    - term
    - granted (true/false)
3. `POST log_request`
    -term
    -logLength
    -logTerm
    -leaderCommit
    -entries
4. `POST log_response`
    -
    -

```
curl -X POST http://localhost:8000/vote_request -H "Content-Type: application/json" -d '{"term": 1, "log_length": 10, "log_term": 5}'

curl -X POST http://localhost:8000/vote_response -H "Content-Type: application/json" -d '{"term": 1, "granted": false}'
```

Testing HTTP-handlers:
1. `GET current_role`:
    - example return: {"current_role": "leader"}
2. `POST stop`
3. `POST recover`

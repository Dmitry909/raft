# Raft algorithm

From root run:
```
go run app/main.go 8000
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
1. `POST heartbeat`
2. `POST vote_request`. Body contains:
    - sender's term
    - len(log)
    - log's last term (or 0, if log is empty)
3. `POST vote_response`
4. `POST log_request`
5. `POST log_response`

```
curl -X POST http://localhost:8080/vote_request -H "Content-Type: application/json" -d '{"senders_term": 1, "len_log": 10, "log_last_term": 5}'
```
# aws-sts-proof

aws-sts-proof is a lightweight implementation of IAM-based authentication using signed GetCallerIdentity requests.

# build

```bash
git clone https://github.com/udhos/aws-sts-proof
cd aws-sts-proof
./build.sh
```

# run server

```bash
$ aws-sts-proof-server
2026/01/07 15:28:38 registered on port :8080 path /
2026/01/07 15:28:38 registered on port :8080 path /health
2026/01/07 15:28:38 registered on port :8080 path /auth
2026/01/07 15:28:38 listening on port :8080
```

# run client

```bash
$ aws-sts-proof-client
...
{"message":"getCallerIdentity.ARN=arn:aws:iam::111122223333:user/udhos","status":"200","path":"/auth","method":"POST","host":"localhost:8080","serverHostname":"jornada"
```

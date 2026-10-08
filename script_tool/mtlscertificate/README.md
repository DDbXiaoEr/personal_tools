# mtlscertificate

自签 mTLS 证书工具：先建根 CA，再签发服务端 / 客户端证书。

## 依赖

- openssl

## 用法

```bash
# 1. 创建根 CA（默认输出 ./certs，有效期 3650 天）
./create-ca.sh [输出目录] [有效天数] [CN]

# 2. 签发服务端证书
./create-server.sh <CN> [输出目录] [有效天数] [SAN...]

# 3. 签发客户端证书（可多次，不同 CN）
./create-client.sh <CN> [输出目录] [有效天数]
```

示例：

```bash
./create-ca.sh
./create-server.sh api.local
./create-server.sh db.local ./certs 3650 db.local 10.0.0.8
./create-client.sh alice
./create-client.sh bob
```

未指定 SAN 时，服务端默认包含：`CN`、`localhost`、`127.0.0.1`、`::1`。

已有文件不会覆盖，重建请先删对应目录。

## 输出

```
certs/
├── ca/
│   ├── ca.key
│   ├── ca.crt
│   └── ca.srl
├── server/<CN>/
│   ├── server.key
│   ├── server.crt
│   └── fullchain.crt    # 服务端证书 + CA
└── client/<CN>/
    ├── client.key
    └── client.crt
```

## 校验

```bash
openssl verify -CAfile certs/ca/ca.crt certs/server/api.local/server.crt
openssl verify -CAfile certs/ca/ca.crt certs/client/alice/client.crt
```

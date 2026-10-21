# mtlscertificate

自签 mTLS 证书工具：先建根 CA，可选再建二级 CA，再签发服务端 / 客户端证书。

有二级 CA 时，叶子证书由二级 CA 签发；校验仍信任根 CA。

## 依赖

- openssl

## 用法

```bash
# 1. 创建根 CA（默认输出 ./certs，有效期 3650 天）
./create-ca.sh [输出目录] [有效天数] [CN]

# 2. （可选）创建二级 CA
./create-intermediate-ca.sh [输出目录] [有效天数] [CN]

# 3. 签发服务端证书
./create-server.sh <CN> [输出目录] [有效天数] [SAN...]

# 4. 签发客户端证书（可多次，不同 CN；可选 PKCS12 密码，默认空）
./create-client.sh <CN> [输出目录] [有效天数] [PKCS12密码]
```

示例：

```bash
./create-ca.sh
./create-intermediate-ca.sh
./create-server.sh api.local
./create-server.sh db.local ./certs 3650 db.local 10.0.0.8
./create-client.sh alice
./create-client.sh bob ./certs 3650 changeit
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
├── intermediate/              # 可选
│   ├── intermediate.key
│   ├── intermediate.crt
│   ├── intermediate.srl
│   └── chain.crt              # 二级 CA + 根 CA
├── server/<CN>/
│   ├── server.key
│   ├── server.crt
│   └── fullchain.crt          # 服务端证书 +（二级 CA）+ 根 CA
└── client/<CN>/
    ├── client.key
    ├── client.crt
    └── client.p12             # 私钥 + 证书 + 链，密码见第 4 个参数（默认空）
```

## 校验

```bash
# 仅根 CA
openssl verify -CAfile certs/ca/ca.crt certs/server/api.local/server.crt
openssl verify -CAfile certs/ca/ca.crt certs/client/alice/client.crt

# 有二级 CA
openssl verify -CAfile certs/ca/ca.crt -untrusted certs/intermediate/intermediate.crt \
    certs/server/api.local/server.crt
openssl verify -CAfile certs/ca/ca.crt -untrusted certs/intermediate/intermediate.crt \
    certs/client/alice/client.crt
```

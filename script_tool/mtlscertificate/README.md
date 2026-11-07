# mtlscertificate

自签 mTLS 证书工具：先建根 CA，可选再建二级 CA，再签发服务端 / 客户端证书。

有二级 CA 时，叶子证书由二级 CA 签发；校验仍信任根 CA。

## 依赖

- openssl

## 用法

```bash
# 1. 创建根 CA（名称在前，默认输出 ./certs，有效期 3650 天）
./create-ca.sh [CA名称] [输出目录] [有效天数] [CN]

# 2. （可选）创建二级 CA（CA 名称在前）
./create-intermediate-ca.sh [CA名称] [输出目录] [有效天数] [CN]

# 3. 签发服务端证书（CA 名称在前）
./create-server.sh <CA名称> <CN> [输出目录] [有效天数] [SAN...]

# 4. 签发客户端证书（可多次，不同 CN；可选 PKCS12 密码，默认空）
./create-client.sh <CA名称> <CN> [输出目录] [有效天数] [PKCS12密码]

# 5. 校验证书（不传参数则校验 ./certs 下全部证书；多个 CA 时用 CA_NAME 指定）
./verify-cert.sh [证书文件] [输出目录]
```

根 CA 按名称存放在 `<输出目录>/ca/<名称>/`，可创建多个不同名称的 CA：

```bash
./create-ca.sh ca1              # -> ./certs/ca/ca1/
./create-ca.sh ca2 ./certs 3650 # -> ./certs/ca/ca2/
```

签发服务端 / 客户端证书时，第 1 个参数指定 CA 名称，脚本会自己到 `ca/<名称>/` 找根 CA：

```bash
./create-server.sh ca1 api.local
./create-client.sh ca1 alice
```

二级 CA 也按根 CA 名称分目录存放。为指定 CA 创建二级 CA 时，第 1 个参数传 CA 名称：

```bash
./create-intermediate-ca.sh ca1   # -> ./certs/intermediate/ca1/
```

校验证书时用 `CA_NAME` 环境变量指定 CA（只校验该 CA 签发的证书）：

```bash
CA_NAME=ca1 ./verify-cert.sh
```

为兼容旧目录，`<输出目录>/ca/ca.key` 的旧布局仍可直接使用，其二级 CA 仍放在 `<输出目录>/intermediate/`。

示例：

```bash
./create-ca.sh ca1
./create-intermediate-ca.sh ca1
./create-server.sh ca1 api.local
./create-server.sh ca1 db.local ./certs 3650 db.local 10.0.0.8
./create-client.sh ca1 alice
./create-client.sh ca1 bob ./certs 3650 changeit
```

未指定 SAN 时，服务端默认包含：`CN`、`localhost`、`127.0.0.1`、`::1`。

已有文件不会覆盖，重建请先删对应目录。

## 输出

```
certs/
├── ca/<名称>/               # 根 CA，按名称分目录
│   ├── ca.key
│   ├── ca.crt
│   └── ca.srl
├── intermediate/<名称>/       # 可选，按根 CA 名称分目录
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
    └── client.p12             # 私钥 + 证书 + 链，密码见第 5 个参数（默认空）
```

## 校验

```bash
# 校验输出目录下全部服务端 / 客户端证书（链、有效期、私钥匹配、SAN）
./verify-cert.sh [输出目录]

# 校验单个证书
./verify-cert.sh <证书文件> [输出目录]

# 无参数时默认使用 ./certs；只有一个 CA 时自动识别
./verify-cert.sh
```

多个 CA 时需指定 `CA_NAME`，只校验该 CA 签发的证书：

```bash
CA_NAME=ca1 ./verify-cert.sh
```

`verify-cert.sh` 有二级 CA 时会自动带上中间证书；证书已过期、链不匹配或私钥不匹配时退出码非 0。

也可直接使用 openssl：

```bash
# 仅根 CA（示例 CA 名称 ca1）
openssl verify -CAfile certs/ca/ca1/ca.crt certs/server/api.local/server.crt
openssl verify -CAfile certs/ca/ca1/ca.crt certs/client/alice/client.crt

# 有二级 CA
openssl verify -CAfile certs/ca/ca1/ca.crt -untrusted certs/intermediate/ca1/intermediate.crt \
    certs/server/api.local/server.crt
openssl verify -CAfile certs/ca/ca1/ca.crt -untrusted certs/intermediate/ca1/intermediate.crt \
    certs/client/alice/client.crt
```

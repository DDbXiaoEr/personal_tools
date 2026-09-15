# SSL证书自签名快捷生成工具

## 使用方法

### 基本用法
```bash
./generate-ssl.sh
```
默认生成localhost证书，输出到`./ssl`目录。

### 自定义域名
```bash
./generate-ssl.sh example.com ./certs
```

### 参数说明
- 参数1: 域名 (默认: localhost)
- 参数2: 输出目录 (默认: ./ssl)
- 参数3: 证书有效期天数 (默认: 3650)

## 生成的文件
- `ca.crt` - CA证书 (需要信任此证书)
- `ca.key` - CA私钥
- `server.crt` - 服务器证书
- `server.key` - 服务器私钥

## 信任CA证书

### macOS
```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ./ssl/ca.crt
```

### Linux (Ubuntu/Debian)
```bash
sudo cp ./ssl/ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

### Windows
双击`ca.crt` -> 安装证书 -> 本地计算机 -> 将所有证书放入下列存储 -> 受信任的根证书颁发机构

## 使用示例

### Nginx配置
```nginx
server {
    listen 443 ssl;
    server_name localhost;
    
    ssl_certificate /path/to/server.crt;
    ssl_certificate_key /path/to/server.key;
}
```

### Node.js HTTPS
```javascript
const https = require('https');
const fs = require('fs');

const options = {
    key: fs.readFileSync('./ssl/server.key'),
    cert: fs.readFileSync('./ssl/server.crt')
};

https.createServer(options, (req, res) => {
    res.writeHead(200);
    res.end('Hello World!');
}).listen(443);
```
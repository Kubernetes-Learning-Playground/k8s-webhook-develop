## k8s api server webhook 二次开发实践
## k8s-webhook-develop

### 项目丝路

使用k8s插件的webhook功能，部署ValidatingWebhookConfiguration资源对象。

同时支援白名单与黑名单校验镜像的两种方式。

(在yaml/deploy.yaml的环境变量WhITE_OR_BLOCK中设置"white" or "black"。)

1. 白名单：只有列表中的镜像前缀同意创建。

2. 黑名单：只有列表中的镜像前缀拒绝创建。

### 项目部署步骤
1. 进入目录

2. 编译镜像
```
docker run --rm -it -v /root/k8sWebhookPractice:/app -w /app -e GOPROXY=https://goproxy.cn -e CGO_ENABLED=0  golang:1.18.7-alpine3.15 go build -o ./webhookPractice .
```

3. 使用cfssl 建立证书
a.下载CA证书软件
```
# Linux
➜  wget -q --show-progress --https-only --timestamping \
  https://pkg.cfssl.org/R1.2/cfssl_linux-amd64 \
  https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64
➜  chmod +x cfssl_linux-amd64 cfssljson_linux-amd64
➜  sudo mv cfssl_linux-amd64 /usr/local/bin/cfssl
➜  sudo mv cfssljson_linux-amd64 /usr/local/bin/cfssljson
```
b. 创建 CA 证书
```
➜  cat > ca-config.json <<EOF
{
  "signing": {
    "default": {
      "expiry": "8760h"
    },
    "profiles": {
      "server": {
        "usages": ["signing", "key encipherment", "server auth", "client auth"],
        "expiry": "8760h"
      }
    }
  }
}
EOF

➜  cat > ca-csr.json <<EOF
{
    "CN": "kubernetes",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "CN",
            "L": "BeiJing",
            "ST": "BeiJing",
            "O": "k8s",
            "OU": "System"
        }
    ]
}
EOF

➜  cfssl gencert -initca ca-csr.json | cfssljson -bare ca
2021/01/23 16:59:51 [INFO] generating a new CA key and certificate from CSR
2021/01/23 16:59:51 [INFO] generate received request
2021/01/23 16:59:51 [INFO] received CSR
2021/01/23 16:59:51 [INFO] generating key: rsa-2048
2021/01/23 16:59:51 [INFO] encoded CSR
2021/01/23 16:59:51 [INFO] signed certificate with serial number 502715407096434913295607470541422244575186494509
➜  ls -la *.pem
-rw-------  1 ych  staff  1675 Jan 23 17:05 ca-key.pem
-rw-r--r--  1 ych  staff  1310 Jan 23 17:05 ca.pem

```
c.
```
➜  cat > server-csr.json <<EOF
{
  "CN": "admission",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
        "C": "CN",
        "L": "BeiJing",
        "ST": "BeiJing",
        "O": "k8s",
        "OU": "System"
    }
  ]
}
EOF

➜  cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json \
		-hostname=admission-registry.default.svc -profile=server server-csr.json | cfssljson -bare server
2021/01/23 17:08:37 [INFO] generate received request
2021/01/23 17:08:37 [INFO] received CSR
2021/01/23 17:08:37 [INFO] generating key: rsa-2048
2021/01/23 17:08:37 [INFO] encoded CSR
2021/01/23 17:08:37 [INFO] signed certificate with serial number 701199816701013791180179639053450980282079712724
➜  ls -la *.pem
-rw-------  1 ych  staff  1675 Jan 23 17:05 ca-key.pem
-rw-r--r--  1 ych  staff  1310 Jan 23 17:05 ca.pem
-rw-------  1 ych  staff  1675 Jan 23 17:08 server-key.pem
-rw-r--r--  1 ych  staff  1452 Jan 23 17:08 server.pem

```

4.使用生成的 server 证书和私钥创建一个 Secret 对象

```
# 创建Secret
➜  kubectl create secret tls admission-registry-tls \
        --key=server-key.pem \
        --cert=server.pem
secret/admission-registry-tls created
```


5. 启动项目
```
kubectl apply -f deploy.yaml
kubectl apply -f validatewebhook.yaml

[root@vm-0-12-centos yaml]# kubectl apply -f .
deployment.apps/admission-registry created
service/admission-registry created
validatingwebhookconfiguration.admissionregistration.k8s.io/admission-registry created
```

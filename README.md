## k8s api server webhook 二次开发实践
## k8s-webhook-develop

### 项目思路
![](https://github.com/googs1025/k8s-webhook-develop/blob/sidecar_fix/image/%E6%B5%81%E7%A8%8B%E5%9B%BE%20(1).jpg?raw=true)
#### 1. Validate
使用k8s插件的webhook功能，部署ValidatingWebhookConfiguration资源对象。

支持白名单与黑名单校验镜像的两种方式(选其一)。

(在yaml/deploy.yaml的环境变量WhITE_OR_BLOCK中设置"white" or "black"。)

1. 白名单：只有列表中的镜像前缀同意创建。

2. 黑名单：只有列表中的镜像前缀拒绝创建。
   
注：当部署一个deployment时，只能两者选用其一功能。
#### 2. Mutate
使用k8s插件的webhook功能，部署MutatingWebhookConfiguration资源对象。

1.  支持替换(replace)pod image模式 或 边车(sidecar)pod image模式
2.  预计支持sidecar模式的Pod image功能
3.  增加自定义annotation功能

注：当部署一个deployment时，只能三者选用其一功能。

(在yaml/deploy.yaml的环境变量ANNOTATION_OR_IMAGE中设置"image" or "annotation" or "label"。)
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


5. 启动项目(目前支援两个其中一个部署，如果同时部署会报错)
```
# 如果使用validate webhook
kubectl apply -f deploy.yaml
kubectl apply -f validatewebhook.yaml
# 如果使用 mutate webhook
kubectl apply -f deploy.yaml
kubectl apply -f mutatewebhook.yaml

[root@vm-0-12-centos yaml]# kubectl apply -f .
deployment.apps/admission-registry created
service/admission-registry created
validatingwebhookconfiguration.admissionregistration.k8s.io/admission-registry created
```

6. 测试

test.yaml : 主要测试validate webhook 通过白名单的前缀

test1.yaml：主要测试validate webhook 非白名单的前缀 会报错，也可以测试mutate webhook 更换镜像后不会报错。

test3.yaml：主要测试mutate webhook 新增annotation
```
kubectl apply -f test.yaml
kubectl apply -f test1.yaml
kubectl apply -f test3.yaml
```

7. deployment配置

**重点**

目录：yaml/deploy.yaml，主要说明配置文件的环境变量
   
```bigquery
apiVersion: apps/v1
kind: Deployment
metadata:
  name: admission-registry
  labels:
    app: admission-registry
spec:
  replicas: 1
  selector:
    matchLabels:
      app: admission-registry
  template:
    metadata:
      labels:
        app: admission-registry
    spec:
      containers:
        - name: validate
          image: alpine:3.12
          imagePullPolicy: IfNotPresent
          command: ["/app/webhookPractice"]
          env: # 环境变量
            # validate 相关
            - name: WhITE_OR_BLOCK # 使用白名单还是黑名单功能
              value: "white"
            - name: WHITELIST_REGISTRIES # 白名单列表：可自己选填
              value: "docker.io,gcr.io"
            - name: BLACKLIST_REGISTRIES # 黑名单列表：可自己选填
              value: ""
            - name: PORT # 端口
              value: "443"
            # mutate 相关
            - name: MUTATE_OBJECT # 判断mutate是patch镜像功能还是补全annotation
              value: "image" # "image" 或 "annotation" "label"
            - name: ANNOTATION_KEY_VALUE # 可以自定义annotation
              value: "customizeAnnotation:my.practice.admission"
            - name: LABEL_KEY_VALUE # 可以自定义label
              value: "customizeLabel:my.practice.admission"
            - name: MUTATE_PATCH_IMAGE # replace模式的image
              value: "nginx:1.19-alpine"
            - name: MUTATE_PATCH_IMAGE_REPLACE # 区分是replace模式还是sidecar模式
              value: "false"
          ports:
            - containerPort: 443
          volumeMounts:
            - name: webhook-certs
              mountPath: /etc/webhook/certs # 写死的路径
              readOnly: true
            - name: app
              mountPath: /app
            - name: sidecarfile
              mountPath: /etc/webhook/config # 写死的路径
      volumes:
        - name: webhook-certs
          secret: # 把secret 映射到volumes。 最终会转为tls.crt tls.key
            secretName: admission-registry-tls
        - name: app
          hostPath:
            path: /root/k8sWebhookPractice
        - name: sidecarfile # 如果使用sidecar模式，一定要配置此文件
          configMap:
            name: sidecar-injector
---
apiVersion: v1
kind: Service
metadata:
  name: admission-registry
  labels:
    app: admission-registry
spec:
  ports:
    - port: 443
      targetPort: 443
  selector:
    app: admission-registry
```

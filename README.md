# MongoDB exporter

## 声明
基于开源项目mongodb_exporter
https://github.com/percona/mongodb_exporter
该exporter只能采集一个目标，不能动态指定采集目标
要想更改采集目标需要重新启动
启动形式
```
exporter --mongodb.uri=mongodb://127.0.0.1:27017
```


## 目的：
保留原有功能情况下，将exporter改造成可以动态指定采集目标，做到多目标采集

## 用法：
在项目根路径下的conf.yml配置文件中配置mongodb的信息
```
mongo-list:
  - name: mongo-1
    host: 127.0.0.1
    port: 27017
    account:
      - username: root
        password: root
      - username: root-1
        password: root-1
```
原有启动形式启动
```
exporter --mongodb.uri=mongodb://127.0.0.1:27017
```
可以通过url动态指定采集目标
```
http://127.0.0.1:9216/metrics?target={{usr}}:{{psw}}@{{ip}}:{{port}}

#其中，如果想不在url中嵌入password，可以不传password，会在conf.yml文件中获取password
http://127.0.0.1:9216/metrics?target={{usr}}@{{ip}}:{{port}}

#当不指定usr时，以不指定账号密码形式采集
http://127.0.0.1:9216/metrics?target=@{{ip}}:{{port}}

#当不指定target时，以程序启动时所指定的目标作为采集目标
http://127.0.0.1:9216/metrics
```


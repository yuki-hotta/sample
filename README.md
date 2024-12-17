sample
===

First, run etcd server on localhost using docker image.

```
$ docker network create app-tier --driver bridge
$
$ docker run -d --name Etcd-server \
    --network app-tier \
    --publish 2379:2379 \
    --publish 2380:2380 \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    --env ETCD_ADVERTISE_CLIENT_URLS=http://etcd-server:2379 \
    bitnami/etcd:latest
$
$ docker run -it --rm \
    --network app-tier \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    bitnami/etcd:latest etcdctl --endpoints http://etcd-server:2379 put /message Hello
etcd 10:26:10.19 INFO  ==>
etcd 10:26:10.19 INFO  ==> Welcome to the Bitnami etcd container
etcd 10:26:10.19 INFO  ==> Subscribe to project updates by watching https://github.com/bitnami/containers
etcd 10:26:10.19 INFO  ==> Submit issues and feature requests at https://github.com/bitnami/containers/issues
etcd 10:26:10.19 INFO  ==> Upgrade to Tanzu Application Catalog for production environments to access custom-configured and pre-packaged software components. Gain enhanced features, including Software Bill of Materials (SBOM), CVE scan result reports, and VEX documents. To learn more, visit https://bitnami.com/enterprise
etcd 10:26:10.20 INFO  ==>

OK
$
```

Then, start test (it spends 30 seconds into detecting goroutine leaks)
```
$ go test -v ./...
=== RUN   Test_Main
goroutine 20 2024/12/17 19:30:59 get event : &{Key:/message Value:Hello2}, num of goroutine = 15
goroutine 20 2024/12/17 19:31:19 false <nil>
--- PASS: Test_Main (30.35s)
PASS
ok      sample  30.356s
```

While testing, you can put `message` value. 
Each event can be notified by the test program (using LongPoll function), for example
```
$ docker run -it --rm \
    --network app-tier \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    bitnami/etcd:latest etcdctl --endpoints http://etcd-server:2379 put /message Hello2
```

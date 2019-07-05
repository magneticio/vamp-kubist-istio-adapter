# vamp-kubist-istio-adapter
istio adapter for vamp kubist

This adapter receives logentry template from istio mixer

Please read these resources before editing this repo:
https://github.com/istio/istio/wiki/Mixer-Out-Of-Process-Adapter-Walkthrough
https://github.com/repenno/mopa


If you make any changes to proto definitions run:

```shell
./buildwithistio.sh
```
Last stage may fail depending on your change but it is ok.

You can build docker image if everything runs locally.

```shell
./builddocker.sh
```

## Testing locally,

During development, you may need to test regularly. It is possible to configure mixer server to send data locally to your adapter service. Run the following commands for easy testing.

(assuming you followed the out-of-process-adapter tutorial on a MacOS)

While in the root of project folder, open a shell and run mixer server:

```shell
export ADAPTER_PATH=$(pwd)
cd $GOPATH
./out/darwin_amd64/release/mixs server --configStoreURL=fs://$ADAPTER_PATH/adapter/testdata
```

Open another shell and run:

```shell
cd $GOPATH
watch -n 5  ./out/darwin_amd64/release/mixc report --timestamp_attributes request.time="2017-07-04T00:01:10Z" -i request.size=1235 -s request.path="/cart?variant_id=1",destination.service="svc.cluster.local",destination.name="experimentedservice" --stringmap_attributes "request.headers=cookie:ex-1_user=3c445470-721f-48d2-ad4a-02de5d19988c%3Bex-1=dest-1-9191-subset1%3B"
```

And in your project folder run:

```shell
go run main.go
```

You should see the logs appearing on your main process output.

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

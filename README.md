
[![pipeline status](https://gitlab.com/protegear/sbd/badges/master/pipeline.svg)](https://gitlab.com/protegear/sbd/commits/master) [![coverage report](https://gitlab.com/protegear/sbd/badges/master/coverage.svg)](https://gitlab.com/protegear/sbd/commits/master)

# Short Burst Data (Iridium)

*Short Burst Data* (SBD) is used by Iridium to send data from their datacenter to your service. This repository contains a library and a service to receive such data. You can use the library in your code, you can start the `directipserver` by starting the service standalone (or via *docker*) or you can deploy the *directipserver* as a *kubernetes controller* in your cluster.

## Library

This repository implements a Go parser for Iridiums *Short Bust Data*. To use
it you can import it and call the `GetElements` function with a reader:
~~~go
data = "\x01\x008\x01\x00\x1cp\xec\ai300234063904190\x00\x00K\x00\x00U\x9e\xba,\x02\x00\x0ctest message"

el, err := sbd.GetElements(bytes.NewBufferString(data))
~~~

The result is an `InformationBucket` which contains field for the header, location and payload. The latitude and longitude is also transmitted in a `location` field where the values are transformed to positive and negative values.

## Distribution service

If you do not want to use this code as an embedded library, you can use the bundled distribution service. This service needs a configuration for IMEI patterns and backend URL's. When the distributor receives a new SBD packet it will search for all matches of the IMEI in the packet and push a JSON data struct to the configured backend URL's. The JSON data contains all the data from the SBD packet, so its up to the receiver to transform the data to a custom format.

The distributor can be run as a standalone binary or as a controller inside a kubernetes cluster. If you are using it as a controller, it must be run with a serviceaccount which has enough rights to watch services. Take a look at the spec file in the 'kubernetes' folder as an example.

If you have deployed the controller, you can annotate your services like this:
~~~
apiVersion: v1
kind: Service
metadata:
  name: myservice
  labels:
    run: myservice
  annotations:
    protegear.io/directip-imei: ".*"
    protegear.io/directip-port: "8080"
    protegear.io/directip-path: "/"
spec:
  type: ClusterIP
  ports:
  - port: 8080
    protocol: TCP
  selector:
    app: myservice
~~~

The controller will match the imei of incoming messages with the annotated regexp `protegear.io/directip-imei` and if it matches, the controller will post the message as a JSON formatted message. The port and the path can also be annotated; if they are missing the port will be `8080` and the path will be `/`.

You can annotate as many services as you want; you can also annotate them with specific IMEI's. It's up to you.

## Standalone service

First of all you have to compile the service. You need at least Go 1.11 installed. Simply type `make` so build a binary in the `cmd/directipserver/bin` directory.

You can use the service as a standalone daemon. In this case you have to write a configuration file for example:
~~~yaml
- imeipattern: 30.*
  backend: http://localhost:8080/service1
- imeipattern: .*
  backend: https://localhost:8443/service2
  skiptls: true
  header:
    mytoken: 1234
~~~
This configuration would post all IMEI's which start with `30` to be posted to the URL `http://localhost:8080/service1`. All other IMEI's will be posted to the URL `https://localhost:8443/service2` and the distribution service will not check the TLS certificate (use this only in development!). Additional Headers can also be added here.

Now start the distribution service:
~~~sh
$ ./directipserver -config ~/tmp/test.yaml -logformat term 0.0.0.0:8123
INFO[09-02|16:42:23] start service                            stage=test revision=29fb44a3 builddate="2018-09-02 16:30:51+02:00" listen=0.0.0.0:8123 caller=main.go:51
INFO[09-02|16:42:23] start distributor service                worker=1 caller=distributor.go:117
INFO[09-02|16:42:23] start distributor service                worker=3 caller=distributor.go:117
INFO[09-02|16:42:23] start distributor service                worker=2 caller=distributor.go:117
INFO[09-02|16:42:23] start distributor service                worker=0 caller=distributor.go:117
INFO[09-02|16:42:23] change configuration                     stage=test targets="[{ID: IMEIPattern:30.* Backend:http://localhost:8080/service1 SkipTLS:false Header:map[] imeipattern:<nil> client:<nil>} {ID: IMEIPattern:.* Backend:https://localhost:8443/service2 SkipTLS:true Header:map[mytoken:1234] imeipattern:<nil> client:<nil>}]" caller=main.go:71
INFO[09-02|16:42:23] no incluster config, assume standalone mode stage=test caller=main.go:77
INFO[09-02|16:42:23] start distributor service                worker=4 caller=distributor.go:117
INFO[09-02|16:42:23] set config                               config="[{ID: IMEIPattern:30.* Backend:http://localhost:8080/service1 SkipTLS:false Header:map[] imeipattern:0xc00008d2c0 client:0xc000087e30} {ID: IMEIPattern:.* Backend:https://localhost:8443/service2 SkipTLS:true Header:map[mytoken:1234] imeipattern:0xc00008d400 client:0xc000087ec0}]" worker=1 caller=distributor.go:124
~~~

You can omit the `-logformat` option to use json logging.

# Important notice
The *sbd* service always sends an OK-acknowledge back to iridium if the post to the HTTP service was successful. It is up to the receiver of the webservice to store and forward the message. If the service returns a successfull HTTP response code and crashes, the message will be lost because iridium will receive a successfull ack.


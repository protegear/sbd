# Short Burst Data (Iridium)

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

# Important notice
The *sbd* service always sends an OK-acknowledge back to iridium if the post to the HTTP service was successful. It is up to the receiver of the webservice to store and forward the message. If the service returns a successfull HTTP response code and crashes, the message will be lost because iridium will receive a successfull ack.


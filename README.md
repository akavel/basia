apksigner
=========
A lightweight APK signing tool that does not require Android Studio or JRE.

Usage
-----

    $ git clone https://github.com/akavel/apksigner
    $ cd apksigner
    $ go build

    $ ./apksigner -i unsigned.apk -k key.pk8 -c cert.x509.pem -o signed.apk

License
=======
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0). Based on [apksigner](https://github.com/fornwall/apksigner) by Fredrik Fornwall, in turn based on [zip-signer](https://code.google.com/p/zip-signer/) by Ken Ellinwood.

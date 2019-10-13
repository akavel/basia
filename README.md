basia
=====
A builder for Android signed APK files that does not require Android Studio or JRE.

Usage
-----

    $ git clone https://github.com/akavel/basia
    $ cd basia
    $ go build

    $ ./basia -i apk/ -c cert.x509.pem -k key.pk8 -o signed.apk

License
=======
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0). Based on [apksigner](https://github.com/fornwall/apksigner) by Fredrik Fornwall, in turn based on [zip-signer](https://code.google.com/p/zip-signer/) by Ken Ellinwood.

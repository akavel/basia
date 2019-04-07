apksigner
=========
A lightweight APK signing tool that does not require Android Studio or JRE.

Usage
=====
Run as `apksigner [-p password] -k keystore -i input.apk -o output.apk`. This will use the specified keystore (or creating one if necessary) to create a signed and zipaligned output file.

License
=======
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0). Based on [zip-signer](https://code.google.com/p/zip-signer/) by Ken Ellinwood.

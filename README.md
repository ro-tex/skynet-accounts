# skynet-accounts

`skynet-accounts` is a service that stores [Skynet](https://siasky.net) user account data. It uses MongoDB for data
storage. It uses ORY Kratos for the actual account management.

## Setup steps

### The `.env` file

All local secrets are loaded from a `.env` file in the root directory of the project.

Those are (example values):

```.env
COOKIE_DOMAIN="siasky.net"
COOKIE_HASH_KEY="any thirty-two byte string is ok"
COOKIE_ENC_KEY="any thirty-two byte string is ok"
SKYNET_DB_HOST="localhost"
SKYNET_DB_PORT="27017"
SKYNET_DB_USER="username"
SKYNET_DB_PASS="password"
STRIPE_API_KEY="put-your-key-here"
STRIPE_WEBHOOK_SECRET="put-your-secret-here"
```

There are some optional ones, as well:

```.env
ACCOUNTS_JWKS_FILE="/accounts/conf/jwks.json"
OATHKEEPER_ADDR=localhost:4456
SKYNET_ACCOUNTS_LOG_LEVEL=trace
```

Meaning of environment variables:

* COOKIE_HASH_KEY and COOKIE_ENC_KEY are used for securing the cookie which holds the user's JWT token
* SKYNET_DB_HOST, SKYNET_DB_PORT, SKYNET_DB_USER, and SKYNET_DB_PASS tell `accounts` how to connect to the MongoDB
  instance it's supposed to use
* STRIPE_API_KEY, STRIPE_WEBHOOK_SECRET allow us to process user payments made via Stripe
* ACCOUNTS_JWKS_FILE is the file which contains the JWKS `accounts` uses to sign the JWTs it issues for its users. It
  defaults to `/accounts/conf/jwks.json`. This file is required.
* OATHKEEPER_ADDR=localhost:4456
* SKYNET_ACCOUNTS_LOG_LEVEL=trace

### Generating a JWKS

The JSON Web Key Set is a set of cryptographic keys used to sign the JSON Web Tokens `accounts` issues for its users.
These tokens are used to authorise users in front of the service and are required for its operation.

If you don't know how to generate your own JWKS you can use this code snippet:

```go
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/ory/hydra/jwk"
)

func main() {
	gen := jwk.RS256Generator{
		KeyLength: 2048,
	}
	jwks, err := gen.Generate("", "sig")
	if err != nil {
		log.Fatal(err)
	}
	jsonbuf, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		log.Fatal("failed to generate JSON: %s", err)
	}
	os.Stdout.Write(jsonbuf)
}
```

## License

Skynet Accounts uses a custom [License](./LICENSE.md). The Skynet License is a source code license that allows you to
use, modify and distribute the software, but you must preserve the payment mechanism in the software.

For the purposes of complying with our code license, you can use the following Siacoin address:

`fb6c9320bc7e01fbb9cd8d8c3caaa371386928793c736837832e634aaaa484650a3177d6714a`

## Recommended reading

- [JSON and BSON](https://www.mongodb.com/json-and-bson)
- [Using the official MongoDB Go driver](https://vkt.sh/go-mongodb-driver-cookbook/)

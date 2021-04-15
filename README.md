# skynet-accounts

`skynet-accounts` is a service that stores [Skynet](https://siasky.net) user account data. It uses MongoDB for data
storage. It uses ORY Kratos for the actual account management.

## Setup steps

### The `.env` file

All local secrets are loaded from a `.env` file in the root directory of the project.

Those are (example values):

```.env
SKYNET_DB_HOST="localhost"
SKYNET_DB_PORT="27017"
SKYNET_DB_USER="username"
SKYNET_DB_PASS="password"
COOKIE_DOMAIN="siasky.net"
COOKIE_HASH_KEY="any thirty-two byte string is ok"
COOKIE_ENC_KEY="any thirty-two byte string is ok"
STRIPE_API_KEY="put-your-key-here"
STRIPE_WEBHOOK_SECRET="put-your-secret-here"
```

There are some optional ones, as well:

```.env
SKYNET_ACCOUNTS_LOG_LEVEL=trace
KRATOS_ADDR=localhost:4433
OATHKEEPER_ADDR=localhost:4456
```

## License

Skynet Accounts uses a custom [License](./LICENSE.md). The Skynet License is a source
code license that allows you to use, modify and distribute the software, but
you must preserve the payment mechanism in the software.

For the purposes of complying with our code license, you can use the
following Siacoin address:

`fb6c9320bc7e01fbb9cd8d8c3caaa371386928793c736837832e634aaaa484650a3177d6714a`

## Recommended reading

- [JSON and BSON](https://www.mongodb.com/json-and-bson)
- [Using the official MongoDB Go driver](https://vkt.sh/go-mongodb-driver-cookbook/)

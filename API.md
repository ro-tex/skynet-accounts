# API Guide

## General terms

### ORY, Kratos, Oathkeeper, and JWx

While `skynet-accounts` handles account information in the context of a Skynet portal, the baseline account management (
account CRUD, email verification, password resets, etc.) is handled by [ORY](https://www.ory.sh/)
([Kratos](https://www.ory.sh/kratos/) and [Oathkeeper](https://www.ory.sh/oathkeeper/))
to which we often refer to as "Kratos". This also covers the login/logout process and the issuance of JTW tokens. When
we talk about JWT (or JWK, or JWKS)
we mean the tokens issued by ORY.

The workflow of verification follows a simple pattern:

* Oathkeeper exposes a public link on which it shares the public keys with which anyone can verify the validity of the
  JWT tokens it issues.
* `skynet-accounts` fetches those keys and uses them to validate the JWTs it receives in requests.

### User tiers

The tiers communicated by the API are numeric. This is the mapping:

0. Anonymous.
1. Free.
2. Premium 5.
3. Premium 20.
4. Premium 80.

## Auth endpoints

### POST `/login`

Sets the `skynet-jwt` cookie.

* Requires valid JWT: `true`
* GET params: none
* POST params: none
* Returns:
    - 204
    - 400
    - 401 (missing JWT)
    - 500

### POST `/logout`

Removes the `skynet-jwt` cookie.

* Requires valid JWT: `true`
* GET params: none
* POST params: none
* Returns:
    - 204
    - 400
    - 401 (missing JWT)
    - 500

## User endpoints

### GET `/user`

This request combines the "get user data" and "create user" requests - if the users exists in the DB, their data will be
returned. If they don't exist in the DB, an account will be created on the Free tier.

* Requires valid JWT: `true`
* Returns:
    - 200 JSON object
  ```json
  {
    "tier": 1
  }
  ```
    - 401 (missing JWT)
    - 424 (when there is no such user, and we fail to create it)
    - 500 (on any other error)

### PUT `/user` (TODO)

This endpoint allows us to update the user's tier, membership expiration dates, etc.

* Requires valid JWT: `true`
* POST params:
    - TBD
* Returns:
    - 200 JSON object
  ```json
  {
    "tier": 1
  }
  ```
    - 400
    - 401 (missing JWT)
    - 500

### GET `/user/uploads`

Returns a list of all skylinks uploaded by the user.

* Requires valid JWT: `true`
* Returns:
    - 200 JSON Array (TBD)
    - 401 (missing JWT)
    - 424 (when there is no such user, and we fail to create it)
    - 500 (on any other error)

### GET `/user/downloads`

Returns a list of all skylinks downloads by the user.

* Requires valid JWT: `true`
* Returns:
    - 200 JSON Array (TBD)
    - 401 (missing JWT)
    - 424 (when there is no such user, and we fail to create it)
    - 500 (on any other error)

## Reports endpoints

### POST `/track/upload/:skylink`

* Requires valid JWT: `true`
* GET params:
    - skylink: just the skylink hash, no path, no protocol
* POST params: none
* Returns:
    - 204
    - 400
    - 401 (missing JWT)
    - 500

### POST `/track/download/:skylink`

* Requires valid JWT: `true`
* GET params:
    - skylink: just the skylink hash, no path, no protocol
* POST params: none
* Returns:
    - 204
    - 400
    - 401 (missing JWT)
    - 500

### POST `/track/registry/read`

* Requires valid JWT: `true`
* GET params: none
* POST params: none
* Returns:
    - 204
    - 400
    - 401 (missing JWT)
    - 500

### POST `/track/registry/write`

* Requires valid JWT: `true`
* GET params: none
* POST params: none
* Returns:
    - 204
    - 400
    - 401 (missing JWT)
    - 500

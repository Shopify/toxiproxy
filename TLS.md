# TLS

Using Toxiproxy with TLS presents its own challenges.
There are multiple ways how to use Toxiproxy in such a set-up.

## Plain-connection

That means Toxiproxy will just act as a TCP proxy. No patches are necessary. 
TLS handshake will still be performed with actual endpoint. Thus Toxiproxy will
not be able to see (plain-text) traffic but may still apply toxic stuff (like delays) to the flow.

Example `config/toxiproxy.json` 
```json
[
  {
    "name": "quasissl",
    "listen": "[::]:443",
    "upstream": "www.arnes.si:443",
    "enabled": true
  }
]
```

In this case you need to make sure the hostname (www.arnes.si in the example)
points to Toxiproxy IP. You could use hosts file for that with an entry like

```
127.0.0.1 www.arnes.si
```

but that isn't really the best option. A more scalable solution would be to change your DNS server to return fake responses.
Easiest is probably [Coredns](https://coredns.io) with rewrite plugin.

## TLS connection with static certificate

In this mode patched Toxiproxy will terminate the TLS connection and always return the configured certificate.

Example `config/toxiproxy.json` 
```json
[
  {
    "name": "ssl",
    "listen": "[::]:443",
    "upstream": "www.arnes.si:443",
    "enabled": true,
    "tls": {
      "cert": "./cert.crt",
      "key": "./cert.key"
    }
  }
]
```

In this case users will configure different hostname - say toxiproxy.mydomain.org instead of www.arnes.si. If you have
proper X.509 certificate for toxiproxy.mydomain.org (for instance through [Let's Encrypt](https://letsencrypt.org)) everything
will behave fine.

TLS section has an additional option:
"verifyUpstream" that is by default set to false. That is if we are already performing a Man-In-The-Middle attack it doesn't really make much
sense to be cautious about the upstream doing something similar. But you can always do something like:

```json
[
  {
    "name": "ssl",
    "listen": "[::]:443",
    "upstream": "www.arnes.si:443",
    "enabled": true,
    "tls": {
      "cert": "./cert.crt",
      "key": "./cert.key",
      "verifyUpstream": true
    }
  }
]
```

## Dynamic certificates based on SNI

In this mode patched Toxiproxy will observe what the hostname was in the request and use the given certificate as a CA to sign the new (dummy) certificate
that matches this hostname. Currently it will generate 2048 bit RSA keypair for that purpose.

This mode is very similar to the first one (except that Toxiproxy is doing the TLS termination and can see plain-text traffic). You centrally enable transparent
proxying through Toxiproxy this way.
 
An example `config/toxiproxy.json`:

```json
[
  {
    "name": "ssl",
    "listen": "[::]:443",
    "upstream": "www.arnes.si:443",
    "enabled": true,
    "tls": {
      "cert": "./cert.crt",
      "key": "./cert.key",
      "isCA": true
    }
  }
]

Here you need to alter DNS responses and additionally also configure the given CA cert (still passed in the configuration as cert/key) as trusted on all machines
that will be connecting to Toxiproxy. 

When isCA is true Toxiproxy will verify that cert.crt is actually a CA certificate (but you can always create a self-signed one of course).

It is also possible to use "verifyUpstream" setting in this mode.

## Notes

Note that currently there is no option that Toxiproxy would terminate TLS connection and make a plain-text connection to the upstream as (for now) there is no use-case for it.

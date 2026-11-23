# Security Policy

## Security status

The Aemeath CLI has **not been independently audited**. It comes with no warranty (see the MIT licence), and secure configuration and deployment of the projects it creates remain the responsibility of the application developer.

## Supported versions

The CLI is pre-1.0. Security fixes are made on the latest release only.

## Reporting a vulnerability

**Please do not open a public issue for a security problem.**

Report it privately through GitHub: *Security* tab → *Report a vulnerability* on https://github.com/naufalraism/aemeath-cli. Include:

- what you found and which version or commit it affects,
- the command and options that reproduce it,
- the impact you expect.

You will get an acknowledgement as soon as the maintainer can, and we will keep you informed while a fix is prepared. Once a fix is released you are welcome to be credited in the changelog.

A problem in the framework itself (request handling, JWT, sessions, uploads and so on) belongs to the [framework's security policy](https://github.com/naufalraism/aemeath/security/policy). If you are not sure which one it is, report it here.

## Scope

In scope: the `aemeath` command, and the code and configuration it writes into a project (for example a `Dockerfile`, `docker-compose.yml` or `.env.example` that exposes a secret or a port it should not).

Out of scope: vulnerabilities in third-party dependencies (report those upstream; tell us if an upgrade is needed), and issues that need an already-compromised machine.

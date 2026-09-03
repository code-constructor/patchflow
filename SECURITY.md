# Security Policy

Patchflow runs locally, binds to `127.0.0.1` by default, and executes Git
against repositories that the user selects. Path traversal, command injection
through Git arguments, and unsafe rendering of review content are the most
relevant classes of vulnerability.

## Reporting a vulnerability

Please do not open a public issue for security problems. Use GitHub's private
vulnerability reporting on this repository ("Security" tab, "Report a
vulnerability"). Reports are acknowledged as soon as possible; there is no
guaranteed response time for this volunteer-maintained project.

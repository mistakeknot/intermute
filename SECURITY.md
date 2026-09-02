# Security

Report vulnerabilities privately through GitHub's "Report a vulnerability" button on this repository, or by email to the address on the maintainer's GitHub profile. You will get an acknowledgement within a week.

## Threat model

intermute is a coordination server for agents on one machine or one trusted network. Localhost requests are unauthenticated by design; anything else needs a bearer key with a project scope (see README § Auth). It stores agent names, messages, and file-reservation patterns, not file contents. Do not expose the port beyond hosts you trust; messages are delivered to any agent in the same project.

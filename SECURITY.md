# Security

Report vulnerabilities privately through GitHub's "Report a vulnerability" button on this repository, or by email to the address on the maintainer's GitHub profile. You will get an acknowledgement within a week.

## Threat model

intermute is a coordination server for agents on one machine or one trusted network. Localhost requests are unauthenticated by design; anything else needs a bearer key with a project scope (see README § Auth). It stores agent names, messages, and file-reservation patterns, not file contents. Do not expose the port beyond hosts you trust; messages are delivered to any agent in the same project.

## Trust boundary

Loopback callers are trusted as members of the project: any local process can call the API without a bearer key, and it can claim to be any agent by setting `X-Agent-ID` on its requests. An agent's identity is verified only when it also presents the registration token it got back from `POST /api/agents`, as `X-Agent-Token`; intermute checks that token against the claimed ID, but does not require it from localhost by default.

On a shared host, set `allow_localhost_without_auth: false` under `default_policy` in the keys file (see `intermute.keys.yaml.example`) and give every client a project key. That removes the free pass localhost requests otherwise get and forces the same bearer-key + project-scope check non-localhost callers already have.

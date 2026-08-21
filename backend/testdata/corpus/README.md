# Fixture corpus

A very small synthetic AudioMuse corpus used by the backend tests. It is not canonical
knowledge and its content is deliberately fictional, so a test can assert exact counts and
orderings without depending on the real repository.

It is shaped to exercise the loader end to end:

- three nodes across two domains, including one with `relationships: []`;
- edges of three canonical types, giving one node inbound edges from two others;
- one session that nodes cite and one registered session that no node cites (warning);
- one registered source that no node cites (warning);
- one registered source whose locator does not exist (warning).

`schemas/node.schema.yaml` is present only because the loader treats it as a repository root
marker; the required-field list itself is compiled into the parser from the canonical
contract.

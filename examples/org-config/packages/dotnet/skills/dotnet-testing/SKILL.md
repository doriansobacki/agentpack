---
name: dotnet-testing
description: How to structure, name, and run tests in this organization's .NET services. Use when writing or modifying tests in a .NET repository.
---

# .NET testing

- Name tests `Method_Scenario_ExpectedOutcome`.
- Arrange/Act/Assert with a blank line between sections.
- Integration tests live in `*.IntegrationTests` projects and must be runnable with `dotnet test` without external setup.

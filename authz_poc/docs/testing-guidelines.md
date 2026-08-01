# Testing Guidelines

These guidelines apply to tests in this repository.

## Framework

- Use `github.com/stretchr/testify` for assertions.
- Prefer `require` for setup and preconditions that must stop the test immediately.
- Use `assert` only when multiple independent checks in the same test are valuable.

## Test Structure

- Follow the AAA pattern.
- Keep tests organized as:
  - Arrange
  - Act
  - Assert
- Use `t.Run` for every scenario, even when there is only one scenario in a test function.
- Name each `t.Run` so it states the precondition and the expected result.
- Use one scenario per test function by default.
- Avoid table-driven tests unless they clearly improve readability for a small, uniform set of cases.

## Test Content

- Do not place product logic inside tests.
- Keep branching, loops, and data transformation out of test bodies when possible.
- Prefer explicit fixtures and helper constructors over inline procedural setup.
- Make each test read like a specification of one behavior.

## Package Naming

- Use external test packages such as `gomut_test`.
- Test the exported surface from the outside when practical.
- Add internal-package tests only when access to unexported behavior is essential and the API would become less clear otherwise.

## Naming

- Name test functions as `Test{TargetFunctionName}` when testing a specific function.
- Name tests after the behavior under test.
- Use descriptive test names that explain the scenario and expected outcome.

## Scope

- Test public behavior first.
- Add focused unit tests for parser, mutation, and runner helpers when they are hard to observe through public commands.

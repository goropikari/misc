# AGENTS.md

This file defines the standards and guidelines for AI agents working on this project.

## Coding Standards

### Testing Pattern: AAA (Arrange, Act, Assert)

All tests should follow the AAA pattern to ensure clarity and consistency.

1. **Arrange**: Set up the state and inputs required for the test.
   - Initialize mocks, repositories, and services.
   - Create necessary data (e.g., users, memos).
2. **Act**: Execute the specific action or function being tested.
   - This should typically be a single function call.
3. **Assert**: Verify that the action produced the expected outcome.
   - Check return values, errors, and state changes in the repository.
   - Use `testify` for assertions.
   - Use `require` for preconditions and checks where continuing would be unsafe or noisy (e.g., unexpected errors, nil values before field access, setup failures).
   - Use `assert` for independent value checks where collecting multiple failures is useful (e.g., field equality, lengths, boolean state).

### Testing Structure: Subtests

All tests should be wrapped with `t.Run`.

- Write the `t.Run` description in the form `〜の場合、〇〇となる`.
- Keep the description focused on the condition and expected outcome.
- Preserve the AAA pattern inside each `t.Run`.

Example:

```go
func TestExample(t *testing.T) {
    t.Run("入力が有効な場合、処理成功となる", func(t *testing.T) {
        // Arrange
        repo := newFakeRepo()
        service := NewService(repo)
        input := "test-data"

        // Act
        err := service.DoSomething(input)

        // Assert
        require.NoError(t, err)
        assert.Equal(t, "expected", repo.LastValue())
    })
}
```

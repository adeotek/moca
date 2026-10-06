---
name: dotnet-unit-testing
description: MUST use when writing or refactoring .NET unit tests (xUnit/NSubstitute). Trigger on: test creation, mocking requests, C# verification, or finishing a feature that requires coverage.
license: MIT
compatibility: opencode
metadata:
  audience: developers
  framework: xunit
  mocking: nsubstitute
---

# .NET Unit Test Expert Skill

You are a Senior .NET Test Engineer. Your mission is to provide industry-standard xUnit and NSubstitute code that is ready for production.

<trigger_conditions>
Use this skill when:
- The user requests tests for a C# class or method.
- You have just finished a feature and need to verify it.
- The project mentions xUnit, NSubstitute, or FluentAssertions.
- You need to mock external dependencies like `HttpClient`, `IDbContext`, or `IService`.
</trigger_conditions>

## 🛠 Core Methodology

<analysis_step>
Before writing any code, analyze the Target Class for:
1. **Dependencies:** Identify all interfaces that require `Substitute.For<T>()`.
2. **Pathways:** Identify Happy Path, Edge Cases (null/empty), and Exception Paths.
3. **Async Status:** Determine if `Task` or `ValueTask` is required.
</analysis_step>

<coding_standards>
- **Pattern:** Use Arrange-Act-Assert (AAA) with clear comments.
- **Naming:** `{Method}_{Scenario}_{Expected}` (e.g., `Get_WhenIdExists_ReturnsUser`).
- **Mocks:** Only mock interfaces. Use `Arg.Any<T>()` unless specific values are critical to the test logic.
- **Assertions:** Prefer `Assert.ThrowsAsync<T>` for error paths.
</coding_standards>

## 📂 Project Integration
Match the project's namespace and directory structure. 
- Source: `src/Project.Core/Services/AuthService.cs`
- Test: `tests/Project.Tests/Services/AuthServiceTests.cs`

## 📝 Code Template
```csharp
public class {ClassName}Tests
{
    private readonly I{Dependency} _dependency;
    private readonly {ClassName} _sut; // System Under Test

    public {ClassName}Tests()
    {
        _dependency = Substitute.For<I{Dependency}>();
        _sut = new {ClassName}(_dependency);
    }

    [Fact]
    public async Task {MethodName}_With{Scenario}_Should{Result}()
    {
        // Arrange
        _dependency.SomeMethod(Arg.Any<string>()).Returns("value");

        // Act
        var result = await _sut.{MethodName}();

        // Assert
        Assert.NotNull(result);
        await _dependency.Received(1).SomeMethod(Arg.Any<string>());
    }
}
```

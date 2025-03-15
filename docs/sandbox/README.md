# Lua Sandbox APIs

## matchRegex

**Signature:** `matchRegex(pattern, text) -> matched`

Check if the regular expression pattern matches the provided text.

## strSplit

**Signature:** `strSplit(str, sep) -> components`

Split string using provided separator. If a separator is not provided, then "\n" is used by default.

### Example 1

```
strSplit("hello\nworld") -> ["hello", "world"]
```

### Example 2

```
strSplit("hello\nworld", "\n") -> ["hello", "world"]
```

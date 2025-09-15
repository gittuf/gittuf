local patterns = {
  ["Token"] = "[a-zA-Z0-9_-][a-zA-Z0-9_-][a-zA-Z0-9_-][a-zA-Z0-9_-][a-zA-Z0-9_-][a-zA-Z0-9_-][a-zA-Z0-9_-][a-zA-Z0-9_-]+",
  ["Private Key"] = "-----BEGIN [A-Z ]*PRIVATE KEY-----",
  ["Password"] = "[Pp]ass(word)?[=:]%s*[\"']?[^\"'\\n]+[\"']?"
}

local function splitString(str, sep)
  local res = strSplit(str or "", sep or "\n")
  return res or {}
end

local function scanSecretsInStagedFiles()
  local results = {}
  local staged_files = gitGetStagedFilePaths()

  for _, path in ipairs(staged_files) do
    local ok_blob, blobid = pcall(gitGetBlobID, ":", path)
    if not ok_blob or blobid == "" then
      goto continue
    end

    local ok_read, content = pcall(gitReadBlob, blobid)
    if not ok_read or not content then
      goto continue
    end

    local lines = splitString(content)
    for line_num, line in ipairs(lines) do
      for name, pattern in pairs(patterns) do
        local matched = matchRegex(pattern, line)
        if matched then
          if not results[path] then
            results[path] = {}
          end
          table.insert(results[path], {
            type = name,
            line_num = line_num,
            content = line
          })
        end
      end
    end

    ::continue::
  end

  return results
end

local function printResults(results)
  if not results or next(results) == nil then
    print("No secrets found.")
    return
  end

  print("Secrets found:")
  for file, matches in pairs(results) do
    print("\nFile:", file)
    for _, match in ipairs(matches) do
      print(string.format("  - [%s] Line %d: %s", match.type, match.line_num, match.content))
    end
  end
end

local results = scanSecretsInStagedFiles()
printResults(results)
return 0


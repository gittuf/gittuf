// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/gittuf/gittuf/internal/gitinterface"
	lua "github.com/yuin/gopher-lua"
)

// API presents the interface for any API made available within the sandbox.
type API interface {
	GetName() string
	GetSignature() string
	GetHelp() string
	GetExamples() []string
}

// LuaAPI implements the API interface. This is used when the API is implemented
// as a Lua function.
type LuaAPI struct {
	Name           string
	Signature      string
	Help           string
	Examples       []string
	Implementation string
}

func (l *LuaAPI) GetName() string {
	return l.Name
}

func (l *LuaAPI) GetSignature() string {
	return l.Signature
}

func (l *LuaAPI) GetHelp() string {
	return l.Help
}

func (l *LuaAPI) GetExamples() []string {
	return l.Examples
}

// GoAPI implements the API interface. This is used when the API is implemented
// in Go.
type GoAPI struct {
	Name           string
	Signature      string
	Help           string
	Examples       []string
	Implementation lua.LGFunction
}

func (g *GoAPI) GetName() string {
	return g.Name
}

func (g *GoAPI) GetSignature() string {
	return g.Signature
}

func (g *GoAPI) GetHelp() string {
	return g.Help
}

func (g *GoAPI) GetExamples() []string {
	return g.Examples
}

func (l *LuaEnvironment) apiMatchRegex() API {
	return &GoAPI{
		Name:      "matchRegex",
		Signature: "matchRegex(pattern, text) -> matched",
		Help:      "Check if the regular expression pattern matches the provided text.",
		Implementation: func(s *lua.LState) int {
			pattern := s.ToString(1)
			text := s.ToString(2)
			regex, err := regexp.Compile(pattern)
			if err != nil {
				s.Push(lua.LString(fmt.Sprintf("Error: %s", err.Error())))
				return 1
			}
			matched := regex.MatchString(text)
			s.Push(lua.LBool(matched))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiStrSplit() API {
	return &LuaAPI{
		Name:      "strSplit",
		Signature: "strSplit(str, sep) -> components",
		Help:      "Split string using provided separator. If a separator is not provided, then \"\\n\" is used by default.",
		// TODO: check if examples are right
		Examples: []string{
			"strSplit(\"hello\\nworld\") -> [\"hello\", \"world\"]",
			"strSplit(\"hello\\nworld\", \"\\n\") -> [\"hello\", \"world\"]",
		},
		Implementation: `
		function strSplit(str, sep)
			if sep == nil then
				sep = "\n"
			end
			local components = {}
			for component in string.gmatch(str, "([^"..sep.."]+)") do
				table.insert(components, component)
			end
			return components
		end	
		`,
	}
}

func (l *LuaEnvironment) apiGitReadBlob() API {
	return &GoAPI{
		Name:      "gitReadBlob",
		Signature: "gitReadBlob(blobID) -> blob",
		Help:      "Retrieve the bytes of the Git blob specified using its ID from the repository.",
		Examples: []string{
			"gitReadBlob(\"e7fca95377c9bad2418c5df7ab3bab5d652a5309\") -> \"Hello, world!\"",
		},
		Implementation: func(s *lua.LState) int {
			blobID := s.ToString(1)
			hash, err := gitinterface.NewHash(blobID)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			blob, err := l.repository.ReadBlob(hash)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}
			s.Push(lua.LString(blob))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetObjectSize() API {
	return &GoAPI{
		Name:      "gitGetObjectSize",
		Signature: "gitGetObjectSize(objectID) -> size",
		Help:      "Retrieve the size of the Git object specified using its ID from the repository.",
		Examples: []string{
			"gitGetObjectSize(\"e7fca95377c9bad2418c5df7ab3bab5d652a5309\") -> 13",
		},
		Implementation: func(s *lua.LState) int {
			objectID := s.ToString(1)
			hash, err := gitinterface.NewHash(objectID)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			size, err := l.repository.GetObjectSize(hash)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}
			s.Push(lua.LString(strconv.FormatUint(size, 10)))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetTagTarget() API {
	return &GoAPI{
		Name:      "gitGetTagTarget",
		Signature: "gitGetTagTarget(tagID) -> targetID",
		Help:      "Retrieve the ID of the Git object that the tag with the specified ID points to.",
		Examples: []string{
			"gitGetTagTarget(\"f38f261f5df1d393a97aec3a5463017da6c22934\") ->  \"e7fca95377c9bad2418c5df7ab3bab5d652a5309\"",
		},
		Implementation: func(s *lua.LState) int {
			tagID := s.ToString(1)
			hash, err := gitinterface.NewHash(tagID)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			targetID, err := l.repository.GetTagTarget(hash)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}
			s.Push(lua.LString(targetID.String()))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetReference() API {
	return &GoAPI{
		Name:      "gitGetReference",
		Signature: "gitGetReference(ref) -> hash",
		Help:      "Retrieve the tip of the specified Git reference.",
		Examples: []string{
			"gitGetReference(\"main\") -> \"e7fca95377c9bad2418c5df7ab3bab5d652a5309\"",
			"gitGetReference(\"refs/heads/main\") -> \"e7fca95377c9bad2418c5df7ab3bab5d652a5309\"",
			"gitGetReference(\"refs/gittuf/reference-state-log\") -> \"c70885ffc33866dbdfe95d0e10efa6d77c77a43b\"",
		},
		Implementation: func(s *lua.LState) int {
			ref := s.ToString(1)

			hash, err := l.repository.GetReference(ref)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}
			s.Push(lua.LString(hash.String()))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetAbsoluteReference() API {
	return &GoAPI{
		Name:      "gitGetAbsoluteReference",
		Signature: "gitGetAbsoluteReference(ref) -> absoluteRef",
		Help:      "Retried the fully qualified reference path for the specified Git reference.",
		Examples: []string{
			"gitGetAbsoluteReference(\"main\") -> \"refs/heads/main\"",
		},
		Implementation: func(s *lua.LState) int {
			ref := s.ToString(1)

			absoluteRef, err := l.repository.AbsoluteReference(ref)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			s.Push(lua.LString(absoluteRef))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetSymbolicReferenceTarget() API {
	return &GoAPI{
		Name:      "gitGetSymbolicReferenceTarget",
		Signature: "gitGetSymbolicReferenceTarget(ref) -> ref",
		Help:      "Retrieve the name of the Git reference the specified symbolic Git reference is pointing to.",
		Examples: []string{
			"gitGetSymbolicReferenceTarget(\"HEAD\") -> \"refs/heads/main\"",
		},
		Implementation: func(s *lua.LState) int {
			symbolicRef := s.ToString(1)

			ref, err := l.repository.GetSymbolicReferenceTarget(symbolicRef)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			s.Push(lua.LString(ref))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetCommitMessage() API {
	return &GoAPI{
		Name:      "gitGetCommitMessage",
		Signature: "gitGetCommitMessage(commitID) -> message",
		Help:      "Retrieve the message for the specified Git commit.",
		Examples: []string{
			"gitGetCommitMessage(\"e7fca95377c9bad2418c5df7ab3bab5d652a5309\") -> \"Commit message.\"",
		},
		Implementation: func(s *lua.LState) int {
			id := s.ToString(1)
			hash, err := gitinterface.NewHash(id)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			message, err := l.repository.GetCommitMessage(hash)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}
			s.Push(lua.LString(message))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetFilePathsChangedByCommit() API {
	return &GoAPI{
		Name:      "gitGetFilePathsChangedByCommit",
		Signature: "gitGetFilePathsChangedByCommit(commitID) -> paths",
		Help:      "Retrieve a Lua table of file paths changed by the specified Git commit.",
		Examples: []string{
			"gitGetFilePathsChangedByCommit(\"e7fca95377c9bad2418c5df7ab3bab5d652a5309\") -> 2, \"foo/bar\", \"foo/baz\"",
		},
		Implementation: func(s *lua.LState) int {
			commitID := s.ToString(1)
			hash, err := gitinterface.NewHash(commitID)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			paths, err := l.repository.GetFilePathsChangedByCommit(hash)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			resultTable := lua.LTable{}

			for _, path := range paths {
				resultTable.Append(lua.LString(path))
			}
			s.Push(&resultTable)
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetRemoteURL() API {
	return &GoAPI{
		Name:      "gitGetRemoteURL",
		Signature: "gitGetRemoteURL(remote) -> remoteURL",
		Help:      "Retrieve the remote URL for the specified Git remote.",
		Examples: []string{
			"gitGetRemoteURL(\"origin\") -> \"example.com/example/example\"",
		},
		Implementation: func(s *lua.LState) int {
			remote := s.ToString(1)

			url, err := l.repository.GetRemoteURL(remote)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			s.Push(lua.LString(url))
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetStagedFilePaths() API {
	return &GoAPI{
		Name:      "gitGetStagedFilePaths",
		Signature: "gitGetStagedFilePaths() -> paths",
		Help:      "Retrieve a Lua table of file paths that have staged changes (changes in the index).",
		Examples: []string{
			"gitGetStagedFilePaths() -> [\"foo/bar.txt\", \"baz/qux.py\"]",
		},
		Implementation: func(s *lua.LState) int {
			statuses, err := l.repository.Status()
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}

			resultTable := s.NewTable()
			localIndex := 1
			for filePath, fileStatus := range statuses {
				if fileStatus.X != gitinterface.StatusCodeUnmodified && fileStatus.X != gitinterface.StatusCodeIgnored {
					resultTable.RawSetInt(localIndex, lua.LString(filePath))
					localIndex++
				}
			}

			s.Push(resultTable)
			return 1
		},
	}
}

func (l *LuaEnvironment) apiGitGetBlobID() API {
	return &GoAPI{
		Name:      "gitGetBlobID",
		Signature: "gitGetBlobID(ref, path) -> blobID",
		Help:      "Retrieve the blob ID of the file at the given path from a specific reference (commit or staged index).",
		Examples: []string{
			"gitGetBlobID(\":\", \"s.txt\") -> \"abc123...\" (staged)",
			"gitGetBlobID(\"HEAD\", \"s.txt\") -> \"def456...\" (current commit)",
			"gitGetBlobID(\"HEAD~1\", \"s.txt\") -> \"ghi789...\" (previous commit)",
		},
		Implementation: func(s *lua.LState) int {
			ref := s.ToString(1)
			path := s.ToString(2)

			blobID, err := l.repository.GetBlobID(ref, path)
			if err != nil {
				s.Push(lua.LString(err.Error()))
				return 1
			}
			s.Push(lua.LString(blobID.String()))
			return 1
		},
	}
}

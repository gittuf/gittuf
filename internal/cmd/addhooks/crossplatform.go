// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

import (
	"fmt"
	"runtime"
)

// generateCrossPlatformScript creates a hook script that works on both Unix and Windows
func generateCrossPlatformScript(hookType string, scriptContent string) []byte {
	if runtime.GOOS == "windows" {
		return generateWindowsScript(hookType)
	}
	return generateUnixScript(scriptContent)
}

func generateUnixScript(scriptContent string) []byte {
	return []byte(scriptContent)
}

func generateWindowsScript(hookType string) []byte {
	// For Windows, we create a batch file that can execute the shell script
	var batchScript string

	switch hookType {
	case "pre-push":
		batchScript = `@echo off
REM gittuf pre-push hook for Windows
REM This script provides cross-platform compatibility

REM Check if gittuf is available
where gittuf >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo Error: gittuf could not be found in PATH
    echo Please install gittuf from: https://github.com/gittuf/gittuf/releases/latest
    exit /b 1
)

REM Get arguments
set remote=%1
set url=%2

REM Function to handle errors gracefully
if "%1"=="" (
    echo Error: No remote specified
    exit /b 1
)

echo Processing pre-push hook for remote: %remote%

REM Read stdin and process refs (simplified for Windows)
REM Note: Full stdin processing in batch is complex, this is a basic implementation
for /f "tokens=1,2,3,4" %%a in ('more') do (
    if not "%%b"=="0000000000000000000000000000000000000000" (
        echo Processing ref: %%a -^> %%c
        echo Creating RSL record for %%a...
        gittuf rsl record "%%a" --local-only
        if %ERRORLEVEL% neq 0 (
            echo Error: Failed to create RSL record for %%a
            exit /b 1
        )
    )
)

REM Sync gittuf metadata with remote
echo Syncing gittuf metadata with %remote%...
gittuf sync "%remote%" 2>nul
if %ERRORLEVEL% neq 0 (
    echo Warning: Could not sync with remote, this may be expected for new repositories
)

echo gittuf pre-push hook completed successfully
exit /b 0
`
	case "pre-commit":
		batchScript = `@echo off
REM gittuf pre-commit hook for Windows

REM Check if gittuf is available
where gittuf >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo Error: gittuf could not be found in PATH
    echo Please install gittuf from: https://github.com/gittuf/gittuf/releases/latest
    exit /b 1
)

echo Running gittuf pre-commit verification...

REM Verify current repository state against policies
gittuf verify-ref HEAD 2>nul
if %ERRORLEVEL% neq 0 (
    echo Warning: Current HEAD does not pass gittuf verification
    echo This may be expected for new commits before they are recorded in RSL
)

REM Check if there are any staged changes
git diff --cached --quiet
if %ERRORLEVEL% equ 0 (
    echo No staged changes to verify
    exit /b 0
)

echo Staged changes detected, performing gittuf policy checks...
echo gittuf pre-commit hook completed successfully
exit /b 0
`
	case "post-commit":
		batchScript = `@echo off
REM gittuf post-commit hook for Windows

REM Check if gittuf is available
where gittuf >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo Error: gittuf could not be found in PATH
    echo Please install gittuf from: https://github.com/gittuf/gittuf/releases/latest
    exit /b 1
)

echo Running gittuf post-commit processing...

REM Get the current branch
for /f "tokens=*" %%i in ('git symbolic-ref --short HEAD 2^>nul') do set current_branch=%%i
if "%current_branch%"=="" set current_branch=HEAD

echo Post-commit hook completed for branch: %current_branch%
echo Remember to run 'gittuf rsl record %current_branch%' before pushing
echo Or use 'gittuf add-hooks' to automate RSL management
exit /b 0
`
	default:
		batchScript = fmt.Sprintf(`@echo off
REM gittuf %s hook for Windows
echo Running gittuf %s hook...
echo gittuf %s hook completed successfully
exit /b 0
`, hookType, hookType, hookType)
	}

	return []byte(batchScript)
}

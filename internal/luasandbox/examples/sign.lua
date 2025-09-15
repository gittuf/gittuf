local function getParentCommits(c)
    local d=gitReadBlob(c) if not d then return{} end
    local l=strSplit(d,"\n") local r={}
    for _,x in ipairs(l)do
        if matchRegex("^parent ",x)then
            local p=strSplit(x," ") if#p==2 and p[1]=="parent"then table.insert(r,p[2])end
        end
    end
    return r
end

local function isSigned(c)
    local m=gitGetCommitMessage(c)or""
    m = string.gsub(m, "\n", " ")
    return matchRegex(".*Signed-off-by: .+ <.+>", m)
end

function verifyBranchCommits(b)
    local r=gitGetAbsoluteReference(b) local h=gitGetReference(r)
    if not h then print(b.." not found")return end
    print("Checking "..b)
    local q={h} local v={} local u={} local n=0
    while#q>0 do
        local c=table.remove(q)
        if not v[c]then
            v[c]=true n=n+1
            local s=isSigned(c)
            print(c..": "..(s and"SIGNED"or"UNSIGNED"))
            if not s then table.insert(u,c)end
            for _,p in ipairs(getParentCommits(c))do
                if not v[p]then table.insert(q,p)end
            end
        end
    end
    print("Checked "..n)
    if#u>0 then
        print("Missing sign-off:")
        for _,x in ipairs(u)do print(x)end
    else
        print("All signed")
    end
end

ref = gitGetSymbolicReferenceTarget("HEAD")
verifyBranchCommits(ref)
return 0

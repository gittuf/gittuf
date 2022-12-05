# Demo

```bash
gittuf init --root-key ../keys/root --root-expires=10 --targets-key ../keys/targets --targets-expires=10
gittuf new-rule --role-key ../keys/targets --rule-name protect-main --allow-key ../keys/alice.pub --allow-key ../keys/bob.pub --protect-path "git:branch=main"
gittuf new-rule --role-key ../keys/targets --rule-name protect-secret-contents --allow-key ../keys/alice.pub --protect-path "secret/*"
mkdir secret
echo "this-is-for-alice-only" > secret/alice-only.txt
git add secret/
gittuf commit --role protect-main --role-key ../keys/alice --  -m "Initial commit"
gittuf verify state git:branch=main
echo "some day bob may also write here" >> secret/alice-only.txt
git add secret/
gittuf commit --role protect-main --role-key ../keys/alice --  -m "Make Bob hopeful"
gittuf verify state git:branch=main
gittuf verify trusted-state git:branch=main 1583f0e336aca0b68e5e2416a0fbca731d6c9bb9 1f63ddf624fbd91af7cdd65dcb4797b6b2557082
```

```bash
echo "hi i'm bob" >> secret/alice-only.txt
git add secret/
gittuf commit --role protect-main --role-key ../keys/bob --  -m "Bob gives it a go"
gittuf verify state git:branch=main
gittuf verify trusted-state git:branch=main 1f63ddf624fbd91af7cdd65dcb4797b6b2557082 46d02795e98a0698783c08a1e3e4038823f213e7
```

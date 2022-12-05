# Demo

Let's do setup. We'll carve out a namespace that only Alice can write to.

```bash
$ gittuf init --root-key ../keys/root --root-expires=10 --targets-key ../keys/targets --targets-expires=10
$ gittuf new-rule --role-key ../keys/targets --rule-name protect-main --allow-key ../keys/alice.pub --allow-key ../keys/bob.pub --protect-path "git:branch=main"
$ gittuf new-rule --role-key ../keys/targets --rule-name protect-secret-contents --allow-key ../keys/alice.pub --protect-path "secret/*"
```

Now, suppose Alice makes some changes to her namespace.

```bash
$ mkdir secret
$ echo "this is for alice only" > secret/alice-only.txt
$ git add secret/
$ gittuf commit --role protect-main --role-key ../keys/alice --  -m "Initial commit"
$ gittuf verify state git:branch=main
$ echo "some day bob may also write here" >> secret/alice-only.txt
$ git add secret/
$ gittuf commit --role protect-main --role-key ../keys/alice --  -m "Make Bob hopeful"
$ gittuf verify state git:branch=main
Target git:branch=main verified successfully!
$ gittuf verify trusted-state git:branch=main 74c46567326b64d495466d16dc07742332cc8f8f a35f9f4cfcdf2b0752446ce0f9bf08d2e900a964
Changes in state a35f9f4cfcdf2b0752446ce0f9bf08d2e900a964 follow rules specified in state 74c46567326b64d495466d16dc07742332cc8f8f for git:branch=main!
```

So far so good! What happens if Bob writes to Alice's namespace?

```bash
$ cd ..
$ cp -r demo-clean demo-invalid
$ cd demo-invalid
$ echo "hi i'm bob" >> secret/alice-only.txt
$ git add secret/
$ gittuf commit --role protect-main --role-key ../keys/bob --  -m "Bob gives it a go"
$ gittuf verify state git:branch=main
Target git:branch=main verified successfully!
$ gittuf verify trusted-state git:branch=main a35f9f4cfcdf2b0752446ce0f9bf08d2e900a964 61d1262122a57a4420c923f44a213f4620956ae6
Error: unauthorized change to file secret/alice-only.txt
```

Bob can't sign valid metadata that changes anything Alice's namespace!
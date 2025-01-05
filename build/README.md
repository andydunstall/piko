# Building a Release

Github workflows has been configured in such a way that simply pushing a tag will deploy a new version of Piko.

```
git tag v<TAG>
git push origin --tags
```

This will kick off the "Release" workflow on Github.


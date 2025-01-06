# Building a Release

## Git Tags

Github workflows has been configured in such a way that simply pushing a tag will deploy a new version of Piko.

```
git tag v<TAG>
git push origin --tags
```

This will kick off the "Release" workflow on Github.

## Github Releases

New releases may also be built using the Github GUI. From the releases page:

1. Draft a new release
1. Create your new tag under "Choose a tag", choosing "Create new tag <TAG> on publish"
1. Create your release notes or choose "Generate release notes"
1. Give your release a title.
1. Click Publish.

This will generate a new git tag with the tag you've created. It will run the github action to build the artifacts and will then overwrite the release with a new release which contains the binaries attached.


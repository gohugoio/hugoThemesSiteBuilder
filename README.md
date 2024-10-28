
# Hugo themes

This repository contains a list of themes developed by the Hugo community, which can be accessed at [themes.gohugo.io](https://themes.gohugo.io/). For any queries, refer to the [FAQ](#faq) section below.

[![Netlify Status](https://api.netlify.com/api/v1/badges/58968044-3238-424c-b9b6-e0d00733890c/deploy-status)](https://app.netlify.com/sites/hugothemes/deploys)

# Adding a theme

You can use the command <code>hugo new theme <em>THEME_NAME</em></code> to create a new theme.

Then, from the root of your theme's repository, you need to perform the following steps:

* Create a `config.toml` file that specifies the Hugo version(s) supported by your theme. Also, add a `theme.toml` file and include some relevant metadata about the theme ([see below](#theme-configuration)).
* Add a descriptive `README.md` ([see below](#readmemd)).
* Include a screenshot image in `/images/screenshot.{png,jpg}` and a thumbnail image in `/images/tn.{png,jpg}` ([see below](#media)).
* Push the changes.

After making your theme available online, you can include it here by following the steps mentioned below.

1. Fork and clone this repository
2. Add your theme's URL (e.g. `github.com/user/my-blog-theme`) in [themes.txt](https://github.com/gohugoio/hugoThemesSiteBuilder/edit/main/themes.txt) in [lexicographical order](https://en.wikipedia.org/wiki/Lexicographic_order).
3. Write a meaningful commit message (e.g. `Add theme my-blog-theme`).
4. Create a pull request(PR) and ensure that Netlify deploy preview succeeds.

## Theme configuration

Your theme should have a `theme.toml` file in the root directory. This file should contain relevant metadata about the theme and its creator(s). It's important to note that only the `theme.toml` file format is supported. `theme.yaml` or `theme.json` files are not supported currently.


```toml
name = "Theme Name"
license = "MIT"
licenselink = "Link to theme's license"
description = "Theme description"

# The home page of the theme, where the source can be found.
homepage = "https://github.com/gohugoio/gohugoioTheme"

# If you have a running demo of the theme.
demosite = "https://gohugo.io"

tags = ["blog", "company"]
features = ["some", "awesome", "features"]

# If the theme has multiple authors
authors = [
  {name = "Name of author", homepage = "Website of author"},
  {name = "Name of author", homepage = "Website of author"}
]

# If the theme has a single author
[author]
    name = "Your name"
    homepage = "Your website"

# If porting an existing theme
[original]
    author = "Name of original author"
    homepage = "Link to website of original author"
    repo = "Link to source code of original theme"
```

Your theme should also have a configuration file (such as `config.toml`) that specifies the [Hugo versions](https://gohugo.io/hugo-modules/configuration/#module-config-hugoversion) supported by the theme.

```toml
[module]
  [module.hugoVersion]
    extended = true
    min = "0.55.0"
    max = "0.84.2"
```

You may omit the fields `extended`, `min`, or `max`.

## Media

You must provide two images: a thumbnail and a screenshot, which will be used for theme previews. These images should showcase the theme's layout without any browser or device mockups and adhere to the following specifications:

- **Aspect Ratio**: Both the thumbnail and screenshot should have a 3:2 aspect ratio.
- **File mame & location**:
  - Place `screenshot.{png,jpg}` in <code><em>[ThemeDir]</em>/images/screenshot.{png,jpg}</code>.
  - Place `tn.{png,jpg}` in <code><em>[ThemeDir]</em>/images/tn.{png,jpg}</code>.
- **Dimension**:
  - **Screenshot**: minimum size of 1500×1000 px.
  - **Thumbnail**: minimum size of 900×600 px.

Additional media may be provided in the same directory.

## README.md

Your theme's README file (which should be written in Markdown and called `README.md`) serves two purposes. It's content appears in two places, which are:

1. On your theme's detail page on [themes.gohugo.io](https://themes.gohugo.io/) website.
2. On your theme's regular main page at GitHub/GitLab (as usual).

To make your theme more accessible to users across the globe, it would be helpful if you could include an English translation of the README, at the very least.

### Use absolute paths for images
If you are adding images to README, make sure to use absolute file paths instead of relative paths.

Examples:

- Absolute Path: `https://raw.githubusercontent.com/user/repo/branch/images/tn.png`
- Relative Path: `images/tn.png`


Relative paths work great on GitHub/GitLab, but they don't correspond to the directory structure of [themes.gohugo.io](https://themes.gohugo.io/) website. Therefore, browsers will not be able to display the image if relative path is used.

## Criteria for acceptance of a theme

### 1. Forks must be notably different

A theme based on an existing Hugo theme (aka a fork) must be notably different for it to be considered a separate theme altogether. In such cases, you should list few arguments in `README.md` file mentioning why your theme should be included. 

The definition of _notably different_ can be subjective, but in most cases, it should be clear. Changing a few colors or making a few style changes, for example, does not result in a notably different theme. It would be better if you submit a [pull request](https://docs.github.com/en/pull-requests) to the original theme to include your proposed changes.

### 2. LICENSE

Themes in this repository are accepted only if they come with an Open Source license that allows for the theme to be freely used, modified, and shared. 
To view a list of popular licenses, you can visit [Open Source Initiative](https://opensource.org/licenses) website.

#### 2.1 License of derivative works

If you are porting an existing theme from another platform to Hugo, or if you're forking an existing Hugo theme to incorporate new features and plan to submit the derivative work; it's essential to ensure that the original theme's license requirements are met. 

In case the original theme lacks an Open Source license, you should try to obtain one from the creator of the original work. You cannot add a license on your own. Such derivative work where license of the original work is unclear, will not be accepted.

In any other case, if a submission is found to be in violation of licence of the original work, it will be rejected without further discussion.

### 3. Paid themes

Themes that require payment are not accepted. Themes with READMEs set up as marketing campaigns for other products (e.g. paid version of a free theme) will not be accepted.

## Troubleshooting a failed Netlify deploy preview

The Netlify deploy preview can fail for a variety of reasons. The following steps outline how to troubleshoot such a failure.

1. **Inspect the Log:** examine the associated log and identify the issue.

2. **Fix the issue:** push commits required to fix the issue.

3. **Trigger a new deploy preview:** A new deploy preview can be created as follows.

   Amend the commit in your PR branch

   ```bash
   git commit --amend --no-edit
   ```

   Do a force push

   ```bash
   git push -f
   ```

> [!NOTE]  
> The [themes site](https://themes.gohugo.io/) is scheduled to rebuild every day at UTC 00:00 (HH:MM). Any new theme addition or changes to existing theme will be reflected on the website after the next scheduled rebuild is complete.

> [!NOTE]  
> The build process utilizes metadata (such as config and images) from the latest release of your theme. If you haven't created any releases, it will use information from the most recent commit instead.

> [!WARNING]  
> The buid process uses Go modules. You are requested **not** to delete Git references or tags from your theme's repository. Doing so may cause issues with fetching specific version of a module, leading to errors.

### Common errors and their resolutions

   <details>
   
   <summary>error: no image found for "images/tn</summary>
   
   If your Netlify deploy preview fails with an error like `error: no image found for "images/tn"` then you should check your theme to see whether you have included the screenshots as per the guidelines mentioned [here](#media). 
   If you get this error despite having the screenshots in your theme, it’s likely because the images weren’t part of the latest release when it was created. To resolve this:
   
   1. **Check Your Release**: Make sure the images or other required assets were included in the latest tagged release.
     
   2. **Create a New Release Tag**: If the images were added after the last release, create a new release tag to incorporate them (e.g., `v0.1.1` if the previous release was `v0.1.0`).
   
   3. **Trigger a new deploy preview**: After tagging the latest version, follow the steps mentioned above to trigger a new deploy preview.
   
   </details>

# Outdated themes

According to our current policy, themes that have not been updated within the last 3 years are deemed outdated and are removed. Even if your theme is fully functional, it is recommended that you periodically check and confirm its compatibility with the latest version of Hugo.

# FAQ

**Question:** My theme is flagged as 'old' when it's been updated recently.

**Answer:** We use Hugo Modules to manage the themes -- which is backed by Go Modules. If you have one or more tagged releases (e.g. `v1.0.0`), adhering to [Semantic Versioning](https://semver.org/), we will choose the last version within the current major version. To get rid of that warning you need to tag a new release and wait for us to rebuild the theme site. Note that for unversioned themes, the latest commit gets picked.

**Question:** Can I submit a theme with a repository hosted on git.sr.ht (or any other platform for that matter), given that themes.txt contains links to github.com and gitlab.com?

**Answer:** Yes, it would be accepted if the repository hosted on git.sr.ht (or any other platform) is supported by Go Modules. To confirm the same, you can create a pull request and check if the build process succeeds.



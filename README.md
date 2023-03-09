
# Hugo themes

This repository contains a list of themes developed by the Hugo community, which can be accessed at [themes.gohugo.io](https://themes.gohugo.io/). For any queries, refer to the [FAQ](#faq) section provided below.

[![Netlify Status](https://api.netlify.com/api/v1/badges/58968044-3238-424c-b9b6-e0d00733890c/deploy-status)](https://app.netlify.com/sites/hugothemes/deploys)


# Themes which are out of date are removed

Themes that have not been updated in the past few years are removed as per the current policy. Even if your theme is fully functional, it is recommended that you periodically check and confirm its compatibility with the latest versions of Hugo.

# Adding a theme to the list

You can use the command <code>hugo new theme <em>THEME_NAME</em></code> to create a new theme.

Then, from the root of your theme's repository, you need to perform the following steps:

* Create a `config.toml` file that specifies the Hugo version(s) supported by your theme. Also, add a `theme.toml` file and include some relevant metadata about the theme ([see below](#theme-configuration)).
* Add a descriptive `README.md` ([see below](#readmemd)).
* Include a screenshot image in `/images/screenshot.{png,jpg}` and a thumbnail image in `/images/tn.{png,jpg}` ([see below](#media)).
* Push the changes.

After making your theme available online, you can include it here by following the steps mentioned below.

* Clone this repository: <code>git clone https://github.com/gohugoio/hugoThemesSiteBuilder.git</code>.
* Add your theme's URL (e.g. `github.com/user/my-blog-theme`) to [themes.txt](https://github.com/gohugoio/hugoThemesSiteBuilder/edit/main/themes.txt) in [lexicographical order](https://en.wikipedia.org/wiki/Lexicographic_order).
* Write a meaningful commit message title (e.g. `Add theme my-blog-theme`).
* Create a pull request and ensure that the preview looks good.

> **Note**: If the PR preview does not appear as expected after you have fixed your theme (missing thumbnail image for example), you can trigger a new preview build as follows.

1. Amend the commit in your PR branch
```bash
git commit --amend --no-edit
```

2. Do a force push
```bash
git push -f
```

> **Note**: The site is rebuilt on a daily basis using the list of themes present in this repository. Any changes or modifications you make to an existing theme will be reflected on the site within 24 hours.

## Theme Configuration

Your theme should have a `theme.toml` file in the root directory. This file should contain relevant metadata about the theme and its creator(s). It's important to note that only the `theme.toml` file format is supported. `theme.yaml` or `theme.json` files are not supported.


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
    homepage = "His/Her website"
    repo = "Link to source code of original theme"
```

Your theme should also have a configuration file (such as config.toml) that specifies the [Hugo versions](https://gohugo.io/hugo-modules/configuration/#module-config-hugoversion) supported by the theme.

```toml
[module]
  [module.hugoVersion]
    extended = true
    min = "0.55.0"
    max = "0.84.2"
```

You may omit the fields `extended`, `min`, or `max`.

Theme maintainers are requested **not** to delete Git references or tags from your theme's repository.  Doing so may cause issues with fetching specific version of a module, leading to errors.

## Criteria to be added to this site

### If a fork, it must be notably different

Themes based on another theme (aka forks) must be notably different for us to add it as a new entry to this theme site. You should make this clear in the README; a few arguments as to why we should pick your theme instead of the original? 

The definition of _notably different_ is a little subjective, but in most cases it will be obvious. A new background color is not enough. It would be better for all if you created a PR to add that as an option to the original theme.

### LICENSE

Themes in this repository are accepted only if they come with an Open Source license that allows for the theme to be freely used, modified, and shared. 

To view a list of popular licenses, you can visit [Open Source Initiative](https://opensource.org/licenses) website.

#### License of derivative works

If you are porting an existing theme from another platform to Hugo, or if you're forking an existing Hugo theme to incorporate new features and plan to submit the derivative work, it's essential to ensure that the original theme's licensing requirements are met. 

In cases where the original theme lacks an Open Source license, you cannot add one.

If a submission is found to violate the license of an original work, it will be rejected without further discussion.

### Media

Screenshots are used as theme previews in the list. They should feature a theme's layout (without any browser chrome or device mockups) and have the following dimensions:

* Both the Thumbnail and Screenshot must be in 3:2 aspect ratio.
* Screenshot (`screenshot.png` or (`screenshot.jpg`) should have a dimension of at least 1500×1000 in pixels.
* Thumbnail (`tn.png` or `tn.jpg`) should have a dimension of at least 900×600 in pixels.
* Media must be located in:
    * <code><em>[ThemeDir]</em>/images/screenshot.{png,jpg}</code>
    * <code><em>[ThemeDir]</em>/images/tn.{png,jpg}</code>


Additional media may be provided in that same directory.

### README.md

Your theme's README file (which should be written in Markdown and called `README.md`) serves a double purpose. This is because its content will appear in two places&mdash;i.e., it will appear:

1. On your theme's details page at [themes.gohugo.io](https://themes.gohugo.io/); and
1. At GitHub (as usual), on your theme's regular main page.

To ease accessibility for international users of your theme, please provide at least an English translation of the README.

> **Note**: If you add screenshots to the README, please make use of absolute file paths instead of relative ones like `/images/screenshot.png`. Relative paths work great on GitHub, but they don't correspond to the directory structure of [themes.gohugo.io](https://themes.gohugo.io/). Therefore, browsers will not be able to display screenshots on the theme site under the given (relative) path.

> **Note**: We will not merge themes with READMEs that's set up as marketing campaigns for other products (e.g. paid versions of the free theme).

## FAQ

**Question:** My theme is flagged as 'old' when it's been updated recently.

**Answer:** We use Hugo Modules to manage the themes -- which is backed by Go Modules. If you have one or more tagged releases (e.g. `v1.0.0`), we will choose the last version within the current major version. To get rid of that warning you need to tag a new release and wait for us to rebuild the theme site. Note that for unversioned themes, the latest commit gets picked.


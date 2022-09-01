**Note Note Note:** We're a little backlogged in the Pull Request department and a little short of staff, so we would appreciate if you would hold off on your theme submission until this settles.

# Hugo themes

A collection of themes created by the Hugo community. Builds to [themes.gohugo.io](https://themes.gohugo.io/).

**Have questions?** Have a look at the [FAQ](#faq) first.

[![Netlify Status](https://api.netlify.com/api/v1/badges/58968044-3238-424c-b9b6-e0d00733890c/deploy-status)](https://app.netlify.com/sites/hugothemes/deploys)

# Adding a theme to the list

Create your theme using <code>hugo new theme <em>THEME_NAME</em></code>. In your theme repository:

* Add a `config.toml` with supported Hugo version(s)  and `theme.toml` file to the root of the theme and add some metadata about the theme ([see below](#theme-configuration));
* Add a descriptive `README.md` to the root of the theme ([see below](#readmemd));
* Add `/images/screenshot.png` and `/images/tn.png` ([see below](#media)).

Once your theme repository is on GitHub, you can add it here.

* Clone this repository: <code>git clone https://github.com/gohugoio/hugoThemesSiteBuilder.git</code>;
* Add your theme path (e.g. `github.com/gohugoio/gohugoioTheme`) to [themes.txt](https://github.com/gohugoio/hugoThemesSiteBuilder/edit/main/themes.txt) in lexicographical order.
* Create a Pull Request and verify that the preview looks good.

Note that if the PR preview does not come up as expected (missing thumbnail image etc.), the way you currently can trigger a new preview build after you have fixed your theme is to amend your commit on your PR branch and do a force push:

```bash
git commit --amend --no-edit
git push -f
```
 
## Theme Configuration

You should have a file named `theme.toml` in the root of your theme. This file contains metadata about the theme and its creator or creators. **Only `theme.toml` is accepted, not `theme.yaml` or not `theme.json`**.

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

Your theme should also have a configuration file (e.g. `config.toml`) configuring what [Hugo versions](https://gohugo.io/hugo-modules/configuration/#module-config-hugoversion) the theme supports:

```toml
[module]
  [module.hugoVersion]
    extended = true
    min = "0.55.0"
    max = "0.84.2"
```

Note that you can omit any of the fields `extended`, `min`, or `max`.

Theme maintainers, please do **not** delete Git references or tags from your theme repositories. Otherwise, Hugo Modules will not be able to fetch a specific version of a module, resulting in errors.

## LICENSE

Themes in this repository are accepted only if they come with an Open Source license, that allows for the theme to be freely used, modified, and shared. 

To have a look at popular licenses please visit the [Open Source Initiative](https://opensource.org/licenses) website.

**Note:** When porting an existing theme from another platform to Hugo, or if you are forking another Hugo theme in order to add new features and you wish to submit the derivative work for inclusion at the Hugo Themes Showcase, you really need to make sure that the requirements of the original theme's license are met. 

If a submission is found to violate the LICENSE of an original theme, it will be rejected without further discussion.

## Media

Screenshots are used as theme previews in the list. They should feature a theme's layout (without any browser chrome or device mockups) and have the following dimensions:

* Both the Thumbnail and Screenshot must be in 3:2 aspect ratio.
* Screenshot (`screenshot.png`) should have a dimension of at least 1500×1000 in pixels.
* Thumbnail (`tn.png`) should have a dimension of at least 900×600 in pixels.
* Media must be located in:
    * <code><em>[ThemeDir]</em>/images/screenshot.png</code>
    * <code><em>[ThemeDir]</em>/images/tn.png</code>


Additional media may be provided in that same directory.

## README.md

Your theme's README file (which should be written in Markdown and called `README.md`) serves a double purpose. This is because its content will appear in two places&mdash;i.e., it will appear:

1. On your theme's details page at [themes.gohugo.io](https://themes.gohugo.io/); and
1. At GitHub (as usual), on your theme's regular main page.

To ease accessibility for international users of your theme please provide at least an English translation of the README.

**Note:** If you add screenshots to the README please make use of absolute file paths instead of relative ones like `/images/screenshot.png`. Relative paths work great on GitHub but they don't correspond to the directory structure of [themes.gohugo.io](https://themes.gohugo.io/). Therefore, browsers will not be able to display screenshots on the theme site under the given (relative) path.


## FAQ

**Question:** My theme flagged as 'old' when it's been updated recently.

**Answer:** We use Hugo Modules to manage the themes -- which is backed by Go Modules. If you have one or more tagged releases (e.g. `v1.0.0`), we will choose the last version within the current major version. To get rid of that warning you need to tag a new release and wait for us to rebuild the theme site. Note that for unversioned themes, the latest commit gets picked.


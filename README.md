# hugoThemeSiteBuilder

WORK IN PROGRESS. Do not follow the guide below just yet. https://inspiring-noether-f6fa3f.netlify.app/

[![Netlify Status](https://api.netlify.com/api/v1/badges/faa207e4-92c4-4fd4-8f5d-b8305205fb84/deploy-status)](https://app.netlify.com/sites/inspiring-noether-f6fa3f/deploys)

# Adding a theme to the list

* Create your theme using <code>hugo new theme <em>THEMENAME</em></code>;
* Add a `config.toml` with supported Hugo version(s)  and `theme.toml` file to the root of the theme and add some metadata about the theme (see below);
* Add a descriptive `README.md` to the root of the theme;
* Add `/images/screenshot.png` and `/images/tn.png` ([see below](#media));
* Add your theme path (e.g. `github.com/gohugoio/gohugoioTheme`) to [themes.txt](https://github.com/gohugoio/hugoThemesSiteBuilder/edit/main/themes.txt) in lexicographically order.
* Create a Pull Request and verify that the preview looks good.

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

Note that you can ommit any of the fields `extended`, `min` or `max`.

## LICENSE

Themes in this repository are accepted only if they come with an Open Source license, that allows for the theme to be freely used, modified, and shared. 

To have a look at popular licenses please visit the [Open Source Initiative](https://opensource.org/licenses) website.

**Note:** When porting an existing theme from another platform to Hugo, or if you are forking another Hugo theme in order to add new features and you wish to submit the derivative work for inclusion at the Hugo Themes Showcase, you really need to make sure that the requirements of the original theme's license are met. 

If a submission is found to violate the LICENSE of an original theme, it will be rejected without further discussion.

## Media

Screenshots are used as theme previews in the list, they should feature a theme's layout (without any browser chrome or device mockups) and have the following dimensions:

* Thumbnail should be 900×600 in pixels
* Screenshot should be 1500×1000 in pixels
* Media must be located in:
    * <code><em>[ThemeDir]</em>/images/screenshot.png</code>
    * <code><em>[ThemeDir]</em>/images/tn.png</code>

Additional media may be provided in that same directory.

## README.md

Your theme's README file
(which should be written in Markdown and called `README.md`)
serves a double purpose.
This is because its content will appear in two places&mdash;i.e., it will appear:

1. On your theme's details page at [themes.gohugo.io](https://themes.gohugo.io/); and
1. At GitHub (as usual), on your theme's regular main page.

To ease accessibility for international users of your theme please provide at least an English translation of the README.

**Note:** If you add screenshots to the README please make use of absolute file paths instead of relative ones like `/images/screenshot.png`. Relative paths work great on GitHub but they don't correspond to the directory structure of [themes.gohugo.io](https://themes.gohugo.io/). Therefore, browsers will not be able to display screenshots on the theme site under the given (relative) path.



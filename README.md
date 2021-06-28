# hugoThemeSiteBuilder

[![Netlify Status](https://api.netlify.com/api/v1/badges/faa207e4-92c4-4fd4-8f5d-b8305205fb84/deploy-status)](https://app.netlify.com/sites/inspiring-noether-f6fa3f/deploys)

Work in progress.


* Preview https://inspiring-noether-f6fa3f.netlify.app/
* Add new themes in cmd/hugothemesitebuilder/build/themes.txt
* The new script fetches star info etc. from GitHub and uses that as part of the weight (combined with date)
* The script currently does not build any demo site, but we may consider a build as part of the "theme evaluation" (as in: block themes with lots of errors, or maybe pull them down with weight).
* Similar I have added some notes about checking if `hugo_version` is set and valid.



---
title: "{{ replace .Name "-" " " | title }}"
date: {{ .Date }}
slug: {{ replace .Name "-" " " | title | urlize | lower }}/
description: {{ replace .Name "-" " " | title | lower }}
image: images/default-placeholder.png
caption: Photo by photographer name
categories:
  - category
tags:
  - tag1
  - tag2
draft: false
---
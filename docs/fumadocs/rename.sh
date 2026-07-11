#!/bin/bash
# Move Chinese docs to .zh.mdx
for file in $(find content/docs -type f -name "*.mdx" ! -name "*.en.mdx" ! -name "*.zh.mdx"); do
    mv "$file" "${file%.mdx}.zh.mdx"
done

# Move English docs to .mdx
for file in $(find content/docs -type f -name "*.en.mdx"); do
    mv "$file" "${file%.en.mdx}.mdx"
done

# Move Chinese meta files to meta.zh.json
for file in $(find content/docs -type f -name "meta.json"); do
    mv "$file" "${file%meta.json}meta.zh.json"
done

# Move English meta files to meta.json
for file in $(find content/docs -type f -name "meta.en.json"); do
    mv "$file" "${file%meta.en.json}meta.json"
done

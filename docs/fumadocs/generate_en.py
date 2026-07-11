import os
import glob
import re

def process_file(filepath):
    # If the file is already .en.mdx, skip
    if filepath.endswith('.en.mdx'):
        return
        
    en_filepath = filepath[:-4] + '.en.mdx'
    # If the english version already exists, skip
    if os.path.exists(en_filepath):
        return
        
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
        
    # very simple translation for title/desc to avoid markdown parsing errors
    # this just adds [EN] prefix to the title for placeholder
    def replace_title(match):
        return f"title: [EN] {match.group(1)}"
        
    content = re.sub(r'^title:\s*(.*)$', replace_title, content, flags=re.MULTILINE)
    
    with open(en_filepath, 'w', encoding='utf-8') as f:
        f.write(content)
        
    print(f"Created {en_filepath}")

mdx_files = glob.glob('content/**/*.mdx', recursive=True)
for f in mdx_files:
    process_file(f)

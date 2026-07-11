const fs = require('fs');
const path = require('path');

function walkDir(dir, callback) {
    fs.readdirSync(dir).forEach(f => {
        let dirPath = path.join(dir, f);
        let isDirectory = fs.statSync(dirPath).isDirectory();
        isDirectory ? walkDir(dirPath, callback) : callback(path.join(dir, f));
    });
}

walkDir('content/docs', (filepath) => {
    if (!filepath.endsWith('.en.mdx')) {
        return;
    }
    
    let content = fs.readFileSync(filepath, 'utf-8');
    
    // match `title: "[EN] Title"` or `title: [EN] Title`
    let newContent = content.replace(/^title:\s*"?\[EN\]\s*(.*?)"?$/gm, 'title: "$1"');
    
    if (newContent !== content) {
        fs.writeFileSync(filepath, newContent, 'utf-8');
        console.log(`Removed [EN] from ${filepath}`);
    }
});

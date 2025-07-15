#!/bin/bash

# TeraBox Backend Command Test Script
# This script tests all rclone commands to see which ones work and which show proper error messages

echo "=========================================="
echo "TeraBox Backend Command Test Script"
echo "=========================================="
echo "Testing all rclone commands..."
echo ""

# Test working commands (should work)
echo "=== WORKING COMMANDS (should succeed) ==="

echo "1. Testing 'about' command:"
r=$(~/Desktop/rclone-dev about terabox:)
if [[ $r == *"Total"* ]]; then
    echo "~/Desktop/rclone-dev about terabox: Working"
else
    echo "~/Desktop/rclone-dev about terabox: Failed"
fi

echo "2. Testing 'backend quota' command:"
r=$(~/Desktop/rclone-dev backend quota terabox:)
if [[ $r == *"total"* ]]; then
    echo "~/Desktop/rclone-dev backend quota terabox: Working"
else
    echo "~/Desktop/rclone-dev backend quota terabox: Failed"
fi
echo ""

echo "3. Testing 'backend status' command:"
r=$(~/Desktop/rclone-dev backend status terabox:)
if [[ $r == *"connected"* ]]; then
    echo "~/Desktop/rclone-dev backend status terabox: Working"
else
    echo "~/Desktop/rclone-dev backend status terabox: Failed"
fi
echo ""

echo "4. Testing 'ls' command:"
r=$(~/Desktop/rclone-dev ls terabox:)
if [[ $r == *"errno=0"* ]]; then # only good when DEBUG is turned on
    echo "~/Desktop/rclone-dev ls terabox: Working"
else
    echo "~/Desktop/rclone-dev ls terabox: Failed"
fi
echo ""

echo "5. Testing 'lsd' command:"
r=$(~/Desktop/rclone-dev lsd terabox:)
if [[ $r == *"errno=0"* ]]; then # only good when DEBUG is turned on
    echo "~/Desktop/rclone-dev lsd terabox: Working"
else
    echo "~/Desktop/rclone-dev lsd terabox: Failed"
fi
echo ""

echo "6. Testing 'lsl' command:"
r=$(~/Desktop/rclone-dev lsl terabox:)
if [[ $r == *"errno=0"* ]]; then # only good when DEBUG is turned on
    echo "~/Desktop/rclone-dev lsl terabox: Working"
else
    echo "~/Desktop/rclone-dev lsl terabox: Failed"
fi
echo ""

echo "7. Testing 'lsf' command:"
r=$(~/Desktop/rclone-dev lsf terabox:)
if [[ $r == *"errno=0"* ]]; then # only good when DEBUG is turned on
    echo "~/Desktop/rclone-dev lsf terabox: Working"
else
    echo "~/Desktop/rclone-dev lsf terabox: Failed"
fi
echo ""

echo "8. Testing 'lsjson' command:"
r=$(~/Desktop/rclone-dev lsjson terabox:)
if [[ $r == *"Path"* ]]; then # only good when DEBUG is turned on
    echo "~/Desktop/rclone-dev lsjson terabox: Working"
else
    echo "~/Desktop/rclone-dev lsjson terabox: Failed"
fi
echo ""

echo "9. Testing 'tree' command:"
r=$(~/Desktop/rclone-dev tree terabox:)
if [[ $r == *"├──"* ]]; then
    echo "~/Desktop/rclone-dev tree terabox: Working"
else
    echo "~/Desktop/rclone-dev tree terabox: Failed"
fi
echo ""

echo "10. Testing 'size' command:"
r=$(~/Desktop/rclone-dev size terabox:)
if [[ $r == *"Total:"* ]]; then
    echo "~/Desktop/rclone-dev size terabox: Working"
else
    echo "~/Desktop/rclone-dev size terabox: Failed"
fi
echo ""

echo "11. Testing 'mkdir' command:"
~/Desktop/rclone-dev mkdir terabox:/test-mkdir
r=$(~/Desktop/rclone-dev lsd terabox:)
if [[ $r == *"test-mkdir"* ]]; then
    echo "~/Desktop/rclone-dev mkdir terabox:/test-mkdir: Working"
else
    echo "~/Desktop/rclone-dev mkdir terabox:/test-mkdir: Failed"
fi
echo ""

echo ""
echo "=== UNSUPPORTED COMMANDS (should show proper error messages) ==="

echo "12. Testing 'touch' command (file creation not supported):"
r=$(~/Desktop/rclone-dev touch terabox:/newfile.txt)
if [[ $r == *"optional feature not implemented"* ]]; then
    echo "~/Desktop/rclone-dev touch terabox:/newfile.txt: Working"
else
    echo "~/Desktop/rclone-dev touch terabox:/newfile.txt: Failed"
fi
echo ""

echo "13. Testing 'link' command (sharing not supported):"
r=$(~/Desktop/rclone-dev link terabox:/test-terabox/foo-1/foo.txt)
if [[ $r == *"not supported by TeraBox backend"* ]]; then
    echo "~/Desktop/rclone-dev link terabox:/test-terabox/foo-1/foo.txt: Working"
else
    echo "~/Desktop/rclone-dev link terabox:/test-terabox/foo-1/foo.txt: Failed"
fi
echo ""

echo "14. Fix 'cleanup' command (trash cleanup not supported):"
~/Desktop/rclone-dev cleanup terabox:
echo ""

echo "15. Fix 'purge' command (bulk deletion not supported):"
~/Desktop/rclone-dev purge terabox:/test-terabox
echo ""

echo "16. Fix 'rmdirs' command (directory removal not supported):"
~/Desktop/rclone-dev rmdirs terabox:/test-mkdir
echo ""

echo "17. Fix 'delete' command (file deletion not supported):"
~/Desktop/rclone-dev delete terabox:/test-terabox/foo-1/foo.txt
echo ""

echo "18. Fix 'deletefile' command (file deletion not supported):"
~/Desktop/rclone-dev deletefile terabox:/test-terabox/foo-1/foo.txt
echo ""

echo "19. Fix 'rmdir' command (directory removal not supported):"
~/Desktop/rclone-dev rmdir terabox:/test-mkdir
echo ""

echo ""
echo "=== FILE OPERATION COMMANDS (should show not implemented) ==="

echo "20. Fix 'copy' command (file upload not supported):"
echo "test content" > /tmp/testfile.txt
~/Desktop/rclone-dev copy /tmp/testfile.txt terabox:/testfile.txt
echo ""

echo "21. Fix 'copyto' command (file upload not supported):"
~/Desktop/rclone-dev copyto /tmp/testfile.txt terabox:/testfile2.txt
echo ""

echo "22. Fix 'copyurl' command (file upload not supported):"
~/Desktop/rclone-dev copyurl "https://example-files.online-convert.com/document/txt/example.txt" terabox:/example.txt
echo ""

echo "23. Fix 'rcat' command (file upload not supported):"
echo "test content" | ~/Desktop/rclone-dev rcat terabox:/rcat-test.txt
echo ""

echo "24. Fix 'move' command (file operations not supported):"
~/Desktop/rclone-dev move terabox:/test-terabox/foo-1/foo.txt terabox:/test-terabox/foo-1/foo-moved.txt
echo ""

echo "25. Fix 'moveto' command (file operations not supported):"
~/Desktop/rclone-dev moveto terabox:/test-terabox/foo-1/foo.txt terabox:/test-terabox/foo-1/foo-moveto.txt
echo ""

echo "26. Fix 'sync' command (file operations not supported):"
~/Desktop/rclone-dev sync /tmp terabox:/sync-test
echo ""

echo ""
echo "=== FILE COMPARISON COMMANDS (should show not implemented) ==="

echo "27. Fix 'check' command (file comparison not supported):"
~/Desktop/rclone-dev check /tmp terabox:/test-terabox
echo ""

echo "28. Fix 'checksum' command (file comparison not supported):"
~/Desktop/rclone-dev checksum /tmp terabox:/test-terabox
echo ""

echo ""
echo "=== HASH COMMANDS (should show not implemented) ==="

echo "29. Fix 'hashsum' command (file access not supported):"
~/Desktop/rclone-dev hashsum terabox:/test-terabox
echo ""

echo "30. Fix 'md5sum' command (file access not supported):"
~/Desktop/rclone-dev md5sum terabox:/test-terabox
echo ""

echo "31. Fix 'sha1sum' command (file access not supported):"
~/Desktop/rclone-dev sha1sum terabox:/test-terabox
echo ""

echo ""
echo "=== OTHER COMMANDS ==="

echo "32. Fix 'bisync' command (bidirectional sync not supported):"
~/Desktop/rclone-dev bisync /tmp terabox:/bisync-test
echo ""

echo "33. Implement 'test' command:"
~/Desktop/rclone-dev test terabox:
echo ""
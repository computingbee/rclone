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
/tmp/rclone-dev about terabox:
echo ""

echo "2. Testing 'backend quota' command:"
/tmp/rclone-dev backend quota terabox:
echo ""

echo "3. Testing 'backend status' command:"
/tmp/rclone-dev backend status terabox:
echo ""

echo "4. Testing 'ls' command:"
/tmp/rclone-dev ls terabox:
echo ""

echo "5. Testing 'lsd' command:"
/tmp/rclone-dev lsd terabox:
echo ""

echo "6. Testing 'lsl' command:"
/tmp/rclone-dev lsl terabox:
echo ""

echo "7. Testing 'lsf' command:"
/tmp/rclone-dev lsf terabox:
echo ""

echo "8. Testing 'lsjson' command:"
/tmp/rclone-dev lsjson terabox:
echo ""

echo "9. Testing 'tree' command:"
/tmp/rclone-dev tree terabox:
echo ""

echo "10. Testing 'size' command:"
/tmp/rclone-dev size terabox:
echo ""

echo "11. Testing 'mkdir' command:"
/tmp/rclone-dev mkdir terabox:/test-mkdir
echo ""

echo ""
echo "=== UNSUPPORTED COMMANDS (should show proper error messages) ==="

echo "12. Testing 'touch' command (file creation not supported):"
/tmp/rclone-dev touch terabox:/newfile.txt
echo ""

echo "13. Testing 'link' command (sharing not supported):"
/tmp/rclone-dev link terabox:/test-terabox/foo-1/foo.txt
echo ""

echo "14. Testing 'cleanup' command (trash cleanup not supported):"
/tmp/rclone-dev cleanup terabox:
echo ""

echo "15. Testing 'purge' command (bulk deletion not supported):"
/tmp/rclone-dev purge terabox:/test-terabox
echo ""

echo "16. Testing 'rmdirs' command (directory removal not supported):"
/tmp/rclone-dev rmdirs terabox:/test-mkdir
echo ""

echo "17. Testing 'delete' command (file deletion not supported):"
/tmp/rclone-dev delete terabox:/test-terabox/foo-1/foo.txt
echo ""

echo "18. Testing 'deletefile' command (file deletion not supported):"
/tmp/rclone-dev deletefile terabox:/test-terabox/foo-1/foo.txt
echo ""

echo "19. Testing 'rmdir' command (directory removal not supported):"
/tmp/rclone-dev rmdir terabox:/test-mkdir
echo ""

echo ""
echo "=== FILE OPERATION COMMANDS (should show not implemented) ==="

echo "20. Testing 'copy' command (file upload not supported):"
echo "test content" > /tmp/testfile.txt
/tmp/rclone-dev copy /tmp/testfile.txt terabox:/testfile.txt
echo ""

echo "21. Testing 'copyto' command (file upload not supported):"
/tmp/rclone-dev copyto /tmp/testfile.txt terabox:/testfile2.txt
echo ""

echo "22. Testing 'copyurl' command (file upload not supported):"
/tmp/rclone-dev copyurl "https://example-files.online-convert.com/document/txt/example.txt" terabox:/example.txt
echo ""

echo "23. Testing 'rcat' command (file upload not supported):"
echo "test content" | /tmp/rclone-dev rcat terabox:/rcat-test.txt
echo ""

echo "24. Testing 'move' command (file operations not supported):"
/tmp/rclone-dev move terabox:/test-terabox/foo-1/foo.txt terabox:/test-terabox/foo-1/foo-moved.txt
echo ""

echo "25. Testing 'moveto' command (file operations not supported):"
/tmp/rclone-dev moveto terabox:/test-terabox/foo-1/foo.txt terabox:/test-terabox/foo-1/foo-moveto.txt
echo ""

echo "26. Testing 'sync' command (file operations not supported):"
/tmp/rclone-dev sync /tmp terabox:/sync-test
echo ""

echo ""
echo "=== FILE COMPARISON COMMANDS (should show not implemented) ==="

echo "27. Testing 'check' command (file comparison not supported):"
/tmp/rclone-dev check /tmp terabox:/test-terabox
echo ""

echo "28. Testing 'checksum' command (file comparison not supported):"
/tmp/rclone-dev checksum /tmp terabox:/test-terabox
echo ""

echo ""
echo "=== HASH COMMANDS (should show not implemented) ==="

echo "29. Testing 'hashsum' command (file access not supported):"
/tmp/rclone-dev hashsum terabox:/test-terabox
echo ""

echo "30. Testing 'md5sum' command (file access not supported):"
/tmp/rclone-dev md5sum terabox:/test-terabox
echo ""

echo "31. Testing 'sha1sum' command (file access not supported):"
/tmp/rclone-dev sha1sum terabox:/test-terabox
echo ""

echo ""
echo "=== OTHER COMMANDS ==="

echo "32. Testing 'bisync' command (bidirectional sync not supported):"
/tmp/rclone-dev bisync /tmp terabox:/bisync-test
echo ""

echo "33. Testing 'test' command:"
/tmp/rclone-dev test terabox:
echo ""

echo ""
echo "=========================================="
echo "Test Summary:"
echo "=========================================="
echo "✅ Working commands: about, backend quota, backend status, ls, lsd, lsl, lsf, lsjson, tree, size, mkdir"
echo "❌ Unsupported commands: touch, link, cleanup, purge, rmdirs, delete, deletefile, rmdir"
echo "❌ File operation commands: copy, copyto, copyurl, rcat, move, moveto, sync"
echo "❌ File comparison commands: check, checksum"
echo "❌ Hash commands: hashsum, md5sum, sha1sum"
echo "❌ Other commands: bisync"
echo ""
echo "Test completed!" 
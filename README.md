
Simple Go Language application that scans your FeedWrangler feed list and 
can delete feeds which have not had a post in X days

Also tests each feed to see if the feed URL resolves and if not can delete
the feed

Developed using golang version 1.2

Once you build it run via the command line, using options

-email      Required, FeedWrangler account email address
-password   Required, FeedWrangler account password
-client     Required, FeedWrangler client key from https://feedwrangler.net/developers/clients
-deleteAge  Optional, Delete feeds not updated since X days
-commit     Optional, Commit feed deletes to FeedWrangler

# Mirror Sync GitHub > GitLab

We have a internal GitLab instance where we sync some repositories to run some internal jobs.
This functions trigger the sync of the mirrors in GitLab when we have a push in the repository that is configured.

## Configuration

In the GitHub repository that you want to trigger the sync you need to create a webhook and add the functions url, the secret key
and only subscribe to the push event.

Make sure that the mirror exist in the internal GitLab and it is under the `mattermost/ci-only` namespace.

#!/usr/bin/env python3
"""
Manage X (Twitter) profile and posts using the API.

Usage:
    python3 scripts/x_profile.py --project proseforge --show          # Show current profile
    python3 scripts/x_profile.py --project proseforge --update        # Update profile
    python3 scripts/x_profile.py --project proseforge --post "Hello world!"  # Post a tweet
    python3 scripts/x_profile.py --project proseforge --delete 1234567890    # Delete a tweet

Requirements:
    pip install tweepy
"""

import argparse
import os
import sys
from pathlib import Path

try:
    import tweepy
except ImportError:
    print("Error: tweepy not installed. Run: pip install tweepy")
    sys.exit(1)

SCRIPT_DIR = Path(__file__).parent
ROOT_DIR = SCRIPT_DIR.parent
DEFAULT_PROJECT = 'proseforge'

# Profile settings per project
PROFILE_SETTINGS = {
    'proseforge': {
        'name': 'ProseForge',
        'description': 'AI-powered story creation. Transform articles into engaging narratives. ✨',
        'url': 'https://demo.proseforge.ai',
        'location': 'The Creative Cloud',
    }
}


def log(msg: str, level: str = 'info'):
    """Print formatted log message."""
    prefix = {
        'info': '\033[34m[INFO]\033[0m',
        'ok': '\033[32m[OK]\033[0m',
        'warn': '\033[33m[WARN]\033[0m',
        'error': '\033[31m[ERROR]\033[0m',
    }.get(level, '')
    print(f"{prefix} {msg}")


def load_env(project: str) -> dict[str, str]:
    """Load environment variables from project .env file."""
    env_path = ROOT_DIR / 'projects' / project / '.env'
    env_vars = {}

    if not env_path.exists():
        log(f".env not found: {env_path}", 'error')
        return env_vars

    for line in env_path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        if '=' in line:
            key, _, value = line.partition('=')
            env_vars[key.strip()] = value.strip()

    return env_vars


def get_api_client(env: dict) -> tweepy.API:
    """Create authenticated Twitter API v1.1 client (needed for profile updates)."""
    auth = tweepy.OAuth1UserHandler(
        env.get('X_API_KEY'),
        env.get('X_API_SECRET'),
        env.get('X_ACCESS_TOKEN'),
        env.get('X_ACCESS_SECRET')
    )
    return tweepy.API(auth)


def get_client_v2(env: dict) -> tweepy.Client:
    """Create authenticated Twitter API v2 client."""
    return tweepy.Client(
        consumer_key=env.get('X_API_KEY'),
        consumer_secret=env.get('X_API_SECRET'),
        access_token=env.get('X_ACCESS_TOKEN'),
        access_token_secret=env.get('X_ACCESS_SECRET'),
        bearer_token=env.get('X_BEARER_TOKEN')
    )


def show_profile(project: str) -> bool:
    """Show current X profile."""
    env = load_env(project)

    required = ['X_API_KEY', 'X_API_SECRET', 'X_ACCESS_TOKEN', 'X_ACCESS_SECRET']
    missing = [k for k in required if not env.get(k)]
    if missing:
        log(f"Missing credentials: {', '.join(missing)}", 'error')
        return False

    try:
        api = get_api_client(env)
        user = api.verify_credentials()

        print(f"\nCurrent X Profile (@{user.screen_name}):")
        print("-" * 40)
        print(f"  Name: {user.name}")
        print(f"  Bio: {user.description}")
        print(f"  URL: {user.url}")
        print(f"  Location: {user.location}")
        print(f"  Followers: {user.followers_count}")
        print(f"  Following: {user.friends_count}")
        print(f"  Tweets: {user.statuses_count}")

        return True

    except tweepy.TweepyException as e:
        log(f"API error: {e}", 'error')
        return False


def update_profile(project: str, dry_run: bool = False) -> bool:
    """Update X profile with project settings."""
    env = load_env(project)
    settings = PROFILE_SETTINGS.get(project)

    if not settings:
        log(f"No profile settings defined for project '{project}'", 'error')
        return False

    required = ['X_API_KEY', 'X_API_SECRET', 'X_ACCESS_TOKEN', 'X_ACCESS_SECRET']
    missing = [k for k in required if not env.get(k)]
    if missing:
        log(f"Missing credentials: {', '.join(missing)}", 'error')
        return False

    print(f"\nProfile updates for {project}:")
    print("-" * 40)
    print(f"  Name: {settings['name']}")
    print(f"  Bio: {settings['description']}")
    print(f"  URL: {settings['url']}")
    print(f"  Location: {settings['location']}")

    if dry_run:
        log("Dry run - no changes made", 'info')
        return True

    try:
        api = get_api_client(env)

        # Update profile
        api.update_profile(
            name=settings['name'],
            description=settings['description'],
            url=settings['url'],
            location=settings['location']
        )

        log("Profile updated successfully!", 'ok')

        # Show updated profile
        user = api.verify_credentials()
        print(f"\nUpdated Profile (@{user.screen_name}):")
        print("-" * 40)
        print(f"  Name: {user.name}")
        print(f"  Bio: {user.description}")
        print(f"  URL: {user.url}")
        print(f"  Location: {user.location}")

        return True

    except tweepy.TweepyException as e:
        log(f"API error: {e}", 'error')
        return False


def post_tweet(project: str, message: str, dry_run: bool = False) -> dict | None:
    """
    Post a tweet to X.

    Returns dict with tweet_id on success, None on failure.
    """
    env = load_env(project)

    required = ['X_API_KEY', 'X_API_SECRET', 'X_ACCESS_TOKEN', 'X_ACCESS_SECRET']
    missing = [k for k in required if not env.get(k)]
    if missing:
        log(f"Missing credentials: {', '.join(missing)}", 'error')
        return None

    print(f"\nPosting to X (@{env.get('X_HANDLE', 'unknown')}):")
    print("-" * 40)
    print(f"  Message: {message[:100]}{'...' if len(message) > 100 else ''}")
    print(f"  Length: {len(message)} chars")

    if dry_run:
        log("Dry run - no tweet posted", 'info')
        return {'tweet_id': 'dry-run-id'}

    try:
        client = get_client_v2(env)
        response = client.create_tweet(text=message)

        tweet_id = response.data['id']
        log(f"Tweet posted successfully! ID: {tweet_id}", 'ok')
        log(f"View at: https://x.com/i/status/{tweet_id}", 'info')

        return {'tweet_id': tweet_id}

    except tweepy.TweepyException as e:
        log(f"API error: {e}", 'error')
        return None


def delete_tweet(project: str, tweet_id: str, dry_run: bool = False) -> bool:
    """Delete a tweet by ID."""
    env = load_env(project)

    required = ['X_API_KEY', 'X_API_SECRET', 'X_ACCESS_TOKEN', 'X_ACCESS_SECRET']
    missing = [k for k in required if not env.get(k)]
    if missing:
        log(f"Missing credentials: {', '.join(missing)}", 'error')
        return False

    print(f"\nDeleting tweet: {tweet_id}")

    if dry_run:
        log("Dry run - tweet not deleted", 'info')
        return True

    try:
        client = get_client_v2(env)
        response = client.delete_tweet(tweet_id)

        if response.data.get('deleted'):
            log(f"Tweet {tweet_id} deleted successfully!", 'ok')
            return True
        else:
            log(f"Failed to delete tweet {tweet_id}", 'error')
            return False

    except tweepy.TweepyException as e:
        log(f"API error: {e}", 'error')
        return False


def main():
    parser = argparse.ArgumentParser(
        description='Manage X (Twitter) profile and posts',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s --project proseforge --show              Show current profile
  %(prog)s --project proseforge --update            Update profile
  %(prog)s --project proseforge --post "Hello!"     Post a tweet
  %(prog)s --project proseforge --delete 123456     Delete a tweet by ID
        '''
    )
    parser.add_argument('--project', '-p', default=DEFAULT_PROJECT,
                        help=f'Project name (default: {DEFAULT_PROJECT})')

    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument('--show', action='store_true', help='Show current profile')
    action.add_argument('--update', action='store_true', help='Update profile')
    action.add_argument('--post', metavar='MESSAGE', help='Post a tweet')
    action.add_argument('--delete', metavar='TWEET_ID', help='Delete a tweet by ID')

    parser.add_argument('--dry-run', action='store_true', help='Preview without changes')

    args = parser.parse_args()

    if args.show:
        success = show_profile(args.project)
    elif args.update:
        success = update_profile(args.project, args.dry_run)
    elif args.post:
        result = post_tweet(args.project, args.post, args.dry_run)
        success = result is not None
    elif args.delete:
        success = delete_tweet(args.project, args.delete, args.dry_run)
    else:
        success = False

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""
Manage Instagram Business/Creator account using the Graph API.

Usage:
    python3 scripts/instagram_profile.py --project proseforge --show   # Show profile info
    python3 scripts/instagram_profile.py --project proseforge --post "Caption" --image path/to/image.jpg

Setup:
    1. Create an Instagram Business or Creator account
    2. Connect it to your Facebook Page (via Meta Business Suite)
    3. Create a Facebook App at https://developers.facebook.com/
    4. Get an access token with instagram_basic, instagram_content_publish permissions
    5. Add INSTAGRAM_USER_ID and INSTAGRAM_ACCESS_TOKEN to .env

Note:
    Instagram API does NOT allow updating bio/profile via API.
    Profile changes must be made manually in the Instagram app.
    This script is primarily for viewing profile info and posting content.

Requirements:
    pip install requests
"""

import argparse
import sys
from pathlib import Path

try:
    import requests
except ImportError:
    print("Error: requests not installed. Run: pip install requests")
    sys.exit(1)

SCRIPT_DIR = Path(__file__).parent
ROOT_DIR = SCRIPT_DIR.parent
DEFAULT_PROJECT = 'proseforge'

GRAPH_API_BASE = 'https://graph.facebook.com/v18.0'


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


def get_instagram_account_id(page_id: str, access_token: str) -> str | None:
    """Get Instagram Business Account ID linked to Facebook Page."""
    url = f"{GRAPH_API_BASE}/{page_id}"
    params = {
        'fields': 'instagram_business_account',
        'access_token': access_token
    }

    try:
        response = requests.get(url, params=params)
        response.raise_for_status()
        data = response.json()
        ig_account = data.get('instagram_business_account', {})
        return ig_account.get('id')
    except requests.RequestException as e:
        log(f"API error: {e}", 'error')
        return None


def get_profile_info(ig_user_id: str, access_token: str) -> dict | None:
    """Get Instagram profile information."""
    fields = 'id,username,name,biography,followers_count,follows_count,media_count,profile_picture_url,website'
    url = f"{GRAPH_API_BASE}/{ig_user_id}"
    params = {
        'fields': fields,
        'access_token': access_token
    }

    try:
        response = requests.get(url, params=params)
        response.raise_for_status()
        return response.json()
    except requests.RequestException as e:
        log(f"API error: {e}", 'error')
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                log(f"Instagram error: {error_data.get('error', {}).get('message', 'Unknown')}", 'error')
            except:
                pass
        return None


def show_profile(project: str) -> bool:
    """Show Instagram profile information."""
    env = load_env(project)

    # Try direct Instagram User ID first
    ig_user_id = env.get('INSTAGRAM_USER_ID')
    access_token = env.get('INSTAGRAM_ACCESS_TOKEN') or env.get('FACEBOOK_ACCESS_TOKEN')

    # If no direct ID, try to get it from Facebook Page
    if not ig_user_id:
        page_id = env.get('FACEBOOK_PAGE_ID')
        if page_id and access_token:
            log("INSTAGRAM_USER_ID not set, looking up from Facebook Page...", 'info')
            ig_user_id = get_instagram_account_id(page_id, access_token)
            if ig_user_id:
                log(f"Found Instagram account: {ig_user_id}", 'ok')
                log(f"Add this to .env: INSTAGRAM_USER_ID={ig_user_id}", 'info')

    if not ig_user_id:
        log("INSTAGRAM_USER_ID not set and couldn't find via Facebook Page", 'error')
        log("To get your Instagram User ID:", 'info')
        log("  1. Connect Instagram to your Facebook Page in Meta Business Suite", 'info')
        log("  2. Use Graph API Explorer to query your Page's instagram_business_account", 'info')
        return False

    if not access_token:
        log("INSTAGRAM_ACCESS_TOKEN (or FACEBOOK_ACCESS_TOKEN) not set", 'error')
        return False

    profile = get_profile_info(ig_user_id, access_token)

    if not profile:
        return False

    print(f"\nInstagram Profile: @{profile.get('username', 'Unknown')}")
    print("-" * 40)
    print(f"  ID: {profile.get('id')}")
    print(f"  Name: {profile.get('name', '(not set)')}")
    print(f"  Bio: {profile.get('biography', '(empty)')}")
    print(f"  Website: {profile.get('website', '(not set)')}")
    print(f"  Followers: {profile.get('followers_count', 0)}")
    print(f"  Following: {profile.get('follows_count', 0)}")
    print(f"  Posts: {profile.get('media_count', 0)}")

    log("\nNote: Instagram bio/profile cannot be updated via API.", 'warn')
    log("Use the Instagram app to update your profile.", 'info')

    return True


def post_image(project: str, caption: str, image_url: str, dry_run: bool = False) -> dict | None:
    """
    Post an image to Instagram.

    Note: Instagram API requires the image to be hosted at a public URL.
    You cannot upload local files directly.

    Returns dict with media_id on success, None on failure.
    """
    env = load_env(project)

    ig_user_id = env.get('INSTAGRAM_USER_ID')
    access_token = env.get('INSTAGRAM_ACCESS_TOKEN') or env.get('FACEBOOK_ACCESS_TOKEN')

    if not ig_user_id or not access_token:
        log("INSTAGRAM_USER_ID and access token required", 'error')
        return None

    print(f"\nPosting to Instagram:")
    print("-" * 40)
    print(f"  Image URL: {image_url}")
    print(f"  Caption: {caption[:100]}{'...' if len(caption) > 100 else ''}")

    if dry_run:
        log("Dry run - no post made", 'info')
        return {'media_id': 'dry-run-id'}

    # Step 1: Create media container
    url = f"{GRAPH_API_BASE}/{ig_user_id}/media"
    params = {'access_token': access_token}
    data = {
        'image_url': image_url,
        'caption': caption
    }

    try:
        response = requests.post(url, params=params, data=data)
        response.raise_for_status()
        container = response.json()
        container_id = container.get('id')
        log(f"Created media container: {container_id}", 'info')
    except requests.RequestException as e:
        log(f"Failed to create media container: {e}", 'error')
        return None

    # Step 2: Publish the container
    url = f"{GRAPH_API_BASE}/{ig_user_id}/media_publish"
    data = {'creation_id': container_id}

    try:
        response = requests.post(url, params=params, data=data)
        response.raise_for_status()
        result = response.json()
        media_id = result.get('id')
        log(f"Posted successfully! Media ID: {media_id}", 'ok')
        return {'media_id': media_id}
    except requests.RequestException as e:
        log(f"Failed to publish: {e}", 'error')
        return None


def delete_post(project: str, media_id: str, dry_run: bool = False) -> bool:
    """Delete an Instagram post by media ID."""
    env = load_env(project)

    access_token = env.get('INSTAGRAM_ACCESS_TOKEN') or env.get('FACEBOOK_ACCESS_TOKEN')

    if not access_token:
        log("Access token required in .env", 'error')
        return False

    print(f"\nDeleting Instagram post: {media_id}")

    if dry_run:
        log("Dry run - post not deleted", 'info')
        return True

    url = f"{GRAPH_API_BASE}/{media_id}"
    params = {'access_token': access_token}

    try:
        response = requests.delete(url, params=params)
        response.raise_for_status()
        result = response.json()

        if result.get('success'):
            log(f"Post {media_id} deleted successfully!", 'ok')
            return True
        else:
            log(f"Failed to delete post {media_id}", 'error')
            return False

    except requests.RequestException as e:
        log(f"API error: {e}", 'error')
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                log(f"Instagram error: {error_data.get('error', {}).get('message', 'Unknown')}", 'error')
            except:
                pass
        return False


def main():
    parser = argparse.ArgumentParser(
        description='Manage Instagram Business/Creator account',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s --project proseforge --show                    Show profile info
  %(prog)s --project proseforge --post "Caption" --image https://example.com/image.jpg
  %(prog)s --project proseforge --delete MEDIA_ID         Delete a post by ID

Note: Instagram API does NOT support updating bio/profile programmatically.
      Profile changes must be made in the Instagram app.
        '''
    )
    parser.add_argument('--project', '-p', default=DEFAULT_PROJECT,
                        help=f'Project name (default: {DEFAULT_PROJECT})')

    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument('--show', action='store_true', help='Show profile info')
    action.add_argument('--post', metavar='CAPTION', help='Post an image with caption')
    action.add_argument('--delete', metavar='MEDIA_ID', help='Delete a post by media ID')

    parser.add_argument('--image', metavar='URL', help='Public URL of image to post')
    parser.add_argument('--dry-run', action='store_true', help='Preview without posting')

    args = parser.parse_args()

    if args.show:
        success = show_profile(args.project)
    elif args.post:
        if not args.image:
            log("--image URL required for posting", 'error')
            sys.exit(1)
        result = post_image(args.project, args.post, args.image, args.dry_run)
        success = result is not None
    elif args.delete:
        success = delete_post(args.project, args.delete, args.dry_run)
    else:
        success = False

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()

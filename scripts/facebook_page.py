#!/usr/bin/env python3
"""
Manage Facebook Page using the Graph API.

Usage:
    python3 scripts/facebook_page.py --project proseforge --show       # Show current page info
    python3 scripts/facebook_page.py --project proseforge --update     # Update page info
    python3 scripts/facebook_page.py --project proseforge --update --dry-run  # Preview changes

Setup:
    1. Create a Facebook Page
    2. Go to https://developers.facebook.com/ and create an App
    3. Add "Facebook Login for Business" product
    4. Generate a Page Access Token with pages_manage_metadata permission
    5. Add FACEBOOK_PAGE_ID and FACEBOOK_ACCESS_TOKEN to .env

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

# Page settings per project
PAGE_SETTINGS = {
    'proseforge': {
        'about': 'AI-powered story creation. Transform articles into engaging narratives.',
        'description': 'ProseForge uses AI to help writers create compelling stories from any source material. Whether you\'re transforming articles, blog posts, or research into narratives, ProseForge makes it easy.',
        'website': 'https://proseforge.ai',
        'emails': ['proseforgestories@gmail.com'],
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


def get_page_info(page_id: str, access_token: str) -> dict | None:
    """Get current Facebook Page information."""
    fields = 'id,name,about,description,website,emails,fan_count,link,username,category'
    url = f"{GRAPH_API_BASE}/{page_id}"
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
                log(f"Facebook error: {error_data.get('error', {}).get('message', 'Unknown')}", 'error')
            except:
                pass
        return None


def update_page_info(page_id: str, access_token: str, updates: dict) -> bool:
    """Update Facebook Page information."""
    url = f"{GRAPH_API_BASE}/{page_id}"
    params = {'access_token': access_token}

    try:
        response = requests.post(url, params=params, data=updates)
        response.raise_for_status()
        result = response.json()
        return result.get('success', False)
    except requests.RequestException as e:
        log(f"API error: {e}", 'error')
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                log(f"Facebook error: {error_data.get('error', {}).get('message', 'Unknown')}", 'error')
            except:
                pass
        return False


def show_page(project: str) -> bool:
    """Show current Facebook Page information."""
    env = load_env(project)

    page_id = env.get('FACEBOOK_PAGE_ID')
    access_token = env.get('FACEBOOK_ACCESS_TOKEN')

    if not page_id:
        log("FACEBOOK_PAGE_ID not set in .env", 'error')
        log("To get your Page ID:", 'info')
        log("  1. Go to your Facebook Page", 'info')
        log("  2. Click 'About' in the left menu", 'info')
        log("  3. Scroll down to find 'Page ID'", 'info')
        return False

    if not access_token:
        log("FACEBOOK_ACCESS_TOKEN not set in .env", 'error')
        log("To get a Page Access Token:", 'info')
        log("  1. Go to https://developers.facebook.com/tools/explorer/", 'info')
        log("  2. Select your app", 'info')
        log("  3. Click 'Get Token' > 'Get Page Access Token'", 'info')
        log("  4. Select your page and grant permissions", 'info')
        return False

    page_info = get_page_info(page_id, access_token)

    if not page_info:
        return False

    print(f"\nFacebook Page: {page_info.get('name', 'Unknown')}")
    print("-" * 40)
    print(f"  ID: {page_info.get('id')}")
    print(f"  Username: @{page_info.get('username', 'not set')}")
    print(f"  Category: {page_info.get('category', 'Unknown')}")
    print(f"  About: {page_info.get('about', '(empty)')}")
    print(f"  Description: {page_info.get('description', '(empty)')[:100]}...")
    print(f"  Website: {page_info.get('website', '(empty)')}")
    print(f"  Emails: {', '.join(page_info.get('emails', [])) or '(empty)'}")
    print(f"  Followers: {page_info.get('fan_count', 0)}")
    print(f"  Link: {page_info.get('link', 'N/A')}")

    return True


def update_page(project: str, dry_run: bool = False) -> bool:
    """Update Facebook Page with project settings."""
    env = load_env(project)
    settings = PAGE_SETTINGS.get(project)

    if not settings:
        log(f"No page settings defined for project '{project}'", 'error')
        return False

    page_id = env.get('FACEBOOK_PAGE_ID')
    access_token = env.get('FACEBOOK_ACCESS_TOKEN')

    if not page_id or not access_token:
        log("FACEBOOK_PAGE_ID and FACEBOOK_ACCESS_TOKEN required in .env", 'error')
        return False

    print(f"\nPage updates for {project}:")
    print("-" * 40)
    print(f"  About: {settings['about']}")
    print(f"  Website: {settings['website']}")

    if dry_run:
        log("Dry run - no changes made", 'info')
        return True

    # Build updates dict
    updates = {
        'about': settings['about'],
        'website': settings['website'],
    }

    # Description requires different handling (longer text)
    if 'description' in settings:
        updates['description'] = settings['description']

    success = update_page_info(page_id, access_token, updates)

    if success:
        log("Page updated successfully!", 'ok')

        # Show updated info
        page_info = get_page_info(page_id, access_token)
        if page_info:
            print(f"\nUpdated Page: {page_info.get('name')}")
            print("-" * 40)
            print(f"  About: {page_info.get('about', '(empty)')}")
            print(f"  Website: {page_info.get('website', '(empty)')}")
    else:
        log("Failed to update page", 'error')

    return success


def post_to_page(project: str, message: str, link: str = None, dry_run: bool = False) -> dict | None:
    """
    Post a message to the Facebook Page.

    Returns dict with post_id on success, None on failure.
    """
    env = load_env(project)

    page_id = env.get('FACEBOOK_PAGE_ID')
    access_token = env.get('FACEBOOK_ACCESS_TOKEN')

    if not page_id or not access_token:
        log("FACEBOOK_PAGE_ID and FACEBOOK_ACCESS_TOKEN required in .env", 'error')
        return None

    print(f"\nPosting to Facebook Page:")
    print("-" * 40)
    print(f"  Message: {message[:100]}{'...' if len(message) > 100 else ''}")
    if link:
        print(f"  Link: {link}")

    if dry_run:
        log("Dry run - no post made", 'info')
        return {'post_id': 'dry-run-id'}

    url = f"{GRAPH_API_BASE}/{page_id}/feed"
    params = {'access_token': access_token}
    data = {'message': message}
    if link:
        data['link'] = link

    try:
        response = requests.post(url, params=params, data=data)
        response.raise_for_status()
        result = response.json()
        post_id = result.get('id')
        log(f"Posted successfully! Post ID: {post_id}", 'ok')
        return {'post_id': post_id}
    except requests.RequestException as e:
        log(f"API error: {e}", 'error')
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                log(f"Facebook error: {error_data.get('error', {}).get('message', 'Unknown')}", 'error')
            except:
                pass
        return None


def delete_post(project: str, post_id: str, dry_run: bool = False) -> bool:
    """Delete a Facebook Page post by ID."""
    env = load_env(project)

    access_token = env.get('FACEBOOK_ACCESS_TOKEN')

    if not access_token:
        log("FACEBOOK_ACCESS_TOKEN required in .env", 'error')
        return False

    print(f"\nDeleting Facebook post: {post_id}")

    if dry_run:
        log("Dry run - post not deleted", 'info')
        return True

    url = f"{GRAPH_API_BASE}/{post_id}"
    params = {'access_token': access_token}

    try:
        response = requests.delete(url, params=params)
        response.raise_for_status()
        result = response.json()

        if result.get('success'):
            log(f"Post {post_id} deleted successfully!", 'ok')
            return True
        else:
            log(f"Failed to delete post {post_id}", 'error')
            return False

    except requests.RequestException as e:
        log(f"API error: {e}", 'error')
        if hasattr(e, 'response') and e.response is not None:
            try:
                error_data = e.response.json()
                log(f"Facebook error: {error_data.get('error', {}).get('message', 'Unknown')}", 'error')
            except:
                pass
        return False


def main():
    parser = argparse.ArgumentParser(
        description='Manage Facebook Page',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s --project proseforge --show              Show current page info
  %(prog)s --project proseforge --update            Update page info
  %(prog)s --project proseforge --post "Hello!"     Post to page
  %(prog)s --project proseforge --delete POST_ID    Delete a post by ID
        '''
    )
    parser.add_argument('--project', '-p', default=DEFAULT_PROJECT,
                        help=f'Project name (default: {DEFAULT_PROJECT})')

    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument('--show', action='store_true', help='Show current page info')
    action.add_argument('--update', action='store_true', help='Update page info')
    action.add_argument('--post', metavar='MESSAGE', help='Post a message to the page')
    action.add_argument('--delete', metavar='POST_ID', help='Delete a post by ID')

    parser.add_argument('--link', help='Link to include with post')
    parser.add_argument('--dry-run', action='store_true', help='Preview without changes')

    args = parser.parse_args()

    if args.show:
        success = show_page(args.project)
    elif args.update:
        success = update_page(args.project, args.dry_run)
    elif args.post:
        result = post_to_page(args.project, args.post, args.link, args.dry_run)
        success = result is not None
    elif args.delete:
        success = delete_post(args.project, args.delete, args.dry_run)
    else:
        success = False

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()

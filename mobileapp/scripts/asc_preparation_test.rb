# frozen_string_literal: true
require 'tmpdir'
require 'stringio'
require_relative 'asc-preparation'

class PreparationTest
  def assert_equal(expected, actual)
    raise "Expected #{expected.inspect}, got #{actual.inspect}" unless expected == actual
  end
  def assert_nil(actual) = assert_equal(nil, actual)
  def assert_raises(type)
    begin
      yield
    rescue type
      return
    end
    raise "Expected #{type}"
  end
  def capture_io
    original = $stdout
    $stdout = StringIO.new
    yield
  ensure
    $stdout = original
  end

  def test_long_processing_refreshes_auth_before_expiry_without_creating_a_key
    @token = Struct.new(:expiration, :refreshes) do
      def refresh!
        self.refreshes += 1
        self.expiration = Time.now + 1200
      end
    end.new(Time.now - 1, 0)
    original = @token
    assert_equal original, token
    assert_equal 1, @token.refreshes
    assert_equal original, token
    assert_equal 1, @token.refreshes
  end

  def test_processing_deadline_is_pending_and_journal_is_private
    Dir.mktmpdir do |dir|
      now = 0
      journal = File.join(dir, 'progress.jsonl')
      progress = StoreProgress.new(journal: journal, clock: -> { now }, pause: ->(seconds) { now += seconds })
      calls = 0
      capture_io do
        assert_raises(StoreProgress::Pending) do
          progress.wait('image-id', timeout: 7) do
            calls += 1
            {'attributes' => {'state' => 'UPLOAD_COMPLETE'}}
          end
        end
      end
      assert_equal 7, now
      assert_equal 3, calls
      records = File.readlines(journal).map { |line| JSON.parse(line) }
      assert_equal 'pending', records.last['phase']
      assert_equal 0, File.stat(journal).mode & 0o077
      assert_raises(RuntimeError) { progress.wait('id', timeout: 0) {} }
    end
  end

  def test_wait_returns_actual_ready_or_failed_state_and_resume_does_not_reserve_again
    now = 0
    progress = StoreProgress.new(clock: -> { now }, pause: ->(seconds) { now += seconds })
    states = %w[UPLOAD_COMPLETE APPROVED]
    image = nil
    capture_io { image = progress.wait('id') { {'attributes' => {'state' => states.shift}} } }
    assert_equal 'APPROVED', image.dig('attributes', 'state')
    assert_equal 5, now
    pool = [{'id' => 'existing', 'attributes' => {'referenceName' => 'same-name', 'state' => 'UPLOAD_COMPLETE'}}]
    original_call = method(:call)
    define_singleton_method(:call) do |method, path, _body = nil|
      raise 'Unexpected reservation or delete' unless method == 'get' && path == '/v1/appAssetLibraryImages/existing'
      {'data' => {'id' => 'existing', 'attributes' => {'referenceName' => 'same-name', 'state' => 'APPROVED'}}}
    end
    define_singleton_method(:store_progress) { progress }
    Dir.mktmpdir do |dir|
      file = File.join(dir, 'image.png')
      File.write(file, 'unchanged image')
      result = nil
      capture_io { result = upload('lib', file, 'CREATIVE_ASSETS', 'same-name', pool) }
      assert_equal 'existing', result['id']
    end
  ensure
    define_singleton_method(:call, original_call) if original_call
  end

  def test_processing_pending_keeps_existing_placements_and_does_not_upload
    v = Struct.new(:app_store_state).new('PREPARE_FOR_SUBMISSION')
    rows = [{locale: 'en-US', loc: 'loc', ok: false, placed: [{'placement' => 'existing'}], files: ['one.png'], group: 'group'}]
    define_singleton_method(:shot_rows) { |*| [v, rows] }
    define_singleton_method(:images) { |*| [] }
    define_singleton_method(:shot_name) { |*| 'content-hash' }
    define_singleton_method(:upload) { |*| raise StoreProgress::Pending, 'still processing' }
    define_singleton_method(:call) { |*| raise 'Must not remove placements while replacement is pending' }
    capture_io { assert_raises(StoreProgress::Pending) { place_screenshots('lib', '1.2.3', 'shots', false) } }
  end

  def test_same_file_does_not_accept_failed_or_changed_content_named_assets
    Dir.mktmpdir do |dir|
      file = File.join(dir, '01.png')
      File.write(file, 'image')
      image = {'attributes' => {'referenceName' => shot_name(file), 'state' => 'APPROVED'}}
      assert_equal :hash, same_file(image, file)
      assert_nil same_file({'attributes' => image['attributes'].merge('state' => 'FAILED')}, file)
      File.write(file, 'changed')
      assert_nil same_file(image, file)
    end
  end

  def status_fixture(dir, failed = nil)
    ipa = File.join(dir, 'app.ipa')
    File.write(ipa, 'synthetic package')
    receipt = File.join(dir, 'package.json')
    data = {'schema' => 1, 'upload_allowed' => true, 'version' => '1.2.3', 'build' => '26', 'ipa_sha256' => Digest::SHA256.file(ipa).hexdigest,
            'packages' => %w[com.gtmux.app com.gtmux.app.widget com.gtmux.app.notificationservice].map { |id| {'bundle' => id} }}
    data['ipa_sha256'] = 'wrong' if failed == 'package'
    File.write(receipt, JSON.generate(data))
    expected = LOCALES.to_h { |locale| [locale, {'description' => 'real text'}] }
    v = Struct.new(:id, :app_store_state).new('version-id', 'PREPARE_FOR_SUBMISSION')
    define_singleton_method(:metadata_plan) { |*| expected }
    define_singleton_method(:version_localizations) { |*| [v, LOCALES.to_h { |l| [l, l] }] }
    define_singleton_method(:call) do |_method, path, _body = nil|
      if path.end_with?('/build')
        {'data' => {'attributes' => {'version' => failed == 'build' ? '25' : '26', 'processingState' => 'VALID'}}}
      else
        {'data' => {'attributes' => {'description' => failed == 'metadata' ? 'old text' : 'real text'}}}
      end
    end
    shot_rows = LOCALES.flat_map do |locale|
      [false, true].map do |ipad|
        {locale: locale, ok: failed != 'screenshots', weak: 0, files: Array.new(ipad ? 4 : 7, ipad ? 'ipad-01.png' : '01.png'),
         placed: [{placement: {'attributes' => {'state' => 'ACTIVE'}}}]}
      end
    end
    define_singleton_method(:shot_rows) { |*| [v, shot_rows] }
    define_singleton_method(:library_id) { 'lib' }
    define_singleton_method(:creative_rows) { |*| [v, Array.new(4) { {ok: failed != 'creative', placed: {'attributes' => {'state' => 'ACTIVE'}}} }, []] }
    define_singleton_method(:accessibility_status) do |*|
      {ok: failed != 'accessibility', errors: failed == 'accessibility' ? ['Candidate accessibility review is incomplete'] : [], post_live_claims: []}
    end
    [ipa, receipt]
  end

  def test_api_failure_invalidates_previous_ready_state
    Dir.mktmpdir do |dir|
      state = File.join(dir, 'state.json')
      ipa, receipt = status_fixture(dir)
      capture_io { preparation_status(version: '1.2.3', build: '26', ipa: ipa, receipt: receipt, state: state, metadata_dir: 'metadata', screenshots: 'shots') }
      define_singleton_method(:call) { |*| raise 'network unavailable' }
      assert_raises(RuntimeError) do
        capture_io { preparation_status(version: '1.2.3', build: '26', ipa: ipa, receipt: receipt, state: state, metadata_dir: 'metadata', screenshots: 'shots') }
      end
      assert_equal 'incomplete', JSON.parse(File.read(state))['status']
    end
  end

  def test_each_incomplete_check_prevents_ready_and_state_is_replaced_by_fresh_readback
    Dir.mktmpdir do |dir|
      state = File.join(dir, 'state.json')
      [nil, 'package', 'build', 'metadata', 'screenshots', 'creative', 'accessibility', nil].each do |failed|
        ipa, receipt = status_fixture(dir, failed)
        ok = nil
        capture_io { ok = preparation_status(version: '1.2.3', build: '26', ipa: ipa, receipt: receipt, state: state, metadata_dir: 'metadata', screenshots: 'shots') }
        assert_equal failed.nil?, ok
        result = JSON.parse(File.read(state))
        assert_equal(failed.nil? ? 'ready_for_user_review' : 'incomplete', result['status'])
        assert_equal false, result['submitted']
        assert_equal true, result['user_preview_required']
        assert_equal(failed != 'accessibility', result.dig('accessibility', 'ok'))
        assert_equal 0, File.stat(state).mode & 0o077
      end
    end
  end
end

PreparationTest.instance_methods.grep(/^test_/).sort.each do |name|
  PreparationTest.new.public_send(name)
  puts "passed #{name}"
end

# frozen_string_literal: true
require 'tmpdir'
require_relative 'asc-accessibility'

class AccessibilityTest
  def assert(value, message = 'Assertion failed')
    raise(message) unless value
  end

  def fixture
    values = StoreAccessibility::FEATURES.to_h { |feature| [feature, %w[supportsDarkInterface supportsDifferentiateWithoutColorAlone].include?(feature)] }
    @responses = {
      '/v1/apps?filter[bundleId]=com.gtmux.app&limit=2' => {'data' => [{'id' => 'app', 'attributes' => {'bundleId' => 'com.gtmux.app'}}]},
      '/v1/apps/app/appStoreVersions?filter[platform]=IOS&limit=200' => {'data' => [
        {'id' => 'candidate', 'attributes' => {'versionString' => '1.2.3', 'appStoreState' => 'PREPARE_FOR_SUBMISSION'}},
        {'id' => 'live', 'attributes' => {'versionString' => '1.2.2', 'appStoreState' => 'READY_FOR_SALE'}}]},
      '/v1/appStoreVersions/live/build' => {'data' => {'attributes' => {'version' => '25', 'processingState' => 'VALID'}}},
      '/v1/apps/app/accessibilityDeclarations?filter[state]=PUBLISHED&limit=200' => {'data' => StoreAccessibility::FAMILIES.map do |family|
        {'attributes' => values.merge('state' => 'PUBLISHED', 'deviceFamily' => family)}
      end}
    }
    @helper = StoreAccessibility.new(lambda do |method, path|
      raise 'ASC writes are forbidden' unless method == 'get'
      JSON.parse(JSON.generate(@responses.fetch(path)))
    end)
  end

  def with_review
    fixture
    Dir.mktmpdir do |dir|
      receipt, path = File.join(dir, 'receipt.json'), File.join(dir, 'review.json')
      File.write(receipt, JSON.generate(schema: 1, upload_allowed: true, version: '1.2.3', build: '26'))
      args = {receipt: receipt, version: '1.2.3', build: '26'}
      data = @helper.template(**args)
      data.merge!('reviewed_by' => 'Synthetic reviewer', 'reviewed_at' => Time.now.utc.iso8601,
                  'change_review' => 'Reviewed theme/navigation/text/dependencies; synthetic evidence only',
                  'url_review' => 'Optional URL stays empty')
      data['families'].each do |family, entry|
        entry['environment'] = "Synthetic #{family}; no physical acceptance"
        entry['limitations'] = 'VoiceOver and Larger Text not accepted or claimed'
        entry['common_tasks'].transform_values! { 'Synthetic common-task evidence reference' }
        entry['capabilities'].each do |feature, decision|
          decision['supported'] = data.dig('baseline', 'published', family, feature)
          decision['evidence'] = decision['supported'] ? 'Synthetic matrix and screenshot references' : 'Not evaluated; not claimed'
        end
      end
      save = -> { File.write(path, JSON.generate(data)) }
      save.call
      yield data, path, args, save
    end
  end

  def check(path, args, live: false) = @helper.check(path, **args, live: live)

  def declarations = @responses['/v1/apps/app/accessibilityDeclarations?filter[state]=PUBLISHED&limit=200']['data']

  def test_template_is_unapproved_and_published_values_never_become_candidate_answers
    with_review do |_data, _path, args, _save|
      template = @helper.template(**args)
      assert(template['reviewed_by'].empty?)
      assert(template['families'].values.all? { |entry| entry['capabilities'].values.all? { |decision| decision['supported'].nil? && decision['evidence'].empty? } })
      assert(template.dig('baseline', 'live') == {'version' => '1.2.2', 'build' => '25'})
    end
  end

  def test_unchanged_review_passes_both_families_with_get_only
    with_review { |_data, path, args, _save| assert(check(path, args)[:ok]) }
  end

  def test_modern_version_state_is_preferred_to_deprecated_store_state
    with_review do |_data, path, args, _save|
      version = @responses['/v1/apps/app/appStoreVersions?filter[platform]=IOS&limit=200']['data'].last
      version['attributes']['appVersionState'] = 'READY_FOR_DISTRIBUTION'
      version['attributes'].delete('appStoreState')
      assert(check(path, args)[:ok])
      version['attributes']['appVersionState'] = 'REPLACED_WITH_NEW_VERSION'
      version['attributes']['appStoreState'] = 'READY_FOR_SALE'
      assert(!check(path, args)[:ok])
    end
  end

  def test_historical_ready_versions_and_unreleased_higher_versions_are_not_current
    with_review do |_data, path, args, _save|
      versions = @responses['/v1/apps/app/appStoreVersions?filter[platform]=IOS&limit=200']['data']
      versions.unshift({'id' => 'old', 'attributes' => {'versionString' => '1.2.1', 'appStoreState' => 'READY_FOR_SALE'}})
      versions.unshift({'id' => 'lexical', 'attributes' => {'versionString' => '1.2.10', 'appStoreState' => 'PREPARE_FOR_SUBMISSION'}})
      assert(check(path, args)[:ok])
      versions.last['attributes']['versionString'] = '1.2.9'
      versions << {'id' => 'new', 'attributes' => {'versionString' => '1.2.10', 'appStoreState' => 'READY_FOR_SALE'}}
      @responses['/v1/appStoreVersions/new/build'] = {'data' => {'attributes' => {'version' => '30', 'processingState' => 'VALID'}}}
      assert(@helper.snapshot['live'] == {'version' => '1.2.10', 'build' => '30'})
      assert(!check(path, args)[:ok])
    end
  end

  def test_missing_or_malformed_review_and_missing_evidence_fail_closed
    with_review do |data, path, args, save|
      assert(!check(path + '.missing', args)[:ok])
      original = JSON.generate(data)
      [-> { data['reviewed_by'] = '' }, -> { data['reviewed_at'] = 'invalid' },
       -> { data['reviewed_at'] = (Time.now + 3600).utc.iso8601 }, -> { data['url_review'] = '' },
       -> { data['change_review'] = '' }, -> { data['families'].delete('IPAD') },
       -> { data['families']['IPAD']['common_tasks'].delete('hq') },
       -> { data['families']['IPAD']['capabilities']['supportsVoiceover']['supported'] = 'false' },
       -> { data['families']['IPHONE']['capabilities']['supportsDarkInterface']['evidence'] = '' },
       -> { data['families']['IPHONE']['capabilities']['supportsNewFeature'] = {} },
       -> { data['baseline'] = nil }].each do |mutate|
        data.replace(JSON.parse(original))
        mutate.call
        save.call
        assert(!check(path, args)[:ok])
      end
      File.write(path, '{"private_evidence": "DO_NOT_PRINT"')
      result = check(path, args)
      assert(!result[:ok] && !JSON.generate(result).include?('DO_NOT_PRINT'))
    end
  end

  def test_version_build_and_package_bytes_bind_the_review
    with_review do |_data, path, args, _save|
      assert(!check(path, args.merge(version: '1.2.4'))[:ok])
      assert(!check(path, args.merge(build: '27'))[:ok])
      File.open(args[:receipt], 'a') { |file| file.puts }
      assert(!check(path, args)[:ok])
    end
  end

  def test_current_published_change_requires_reassessment_and_drafts_cannot_pass
    with_review do |_data, path, args, _save|
      declarations.first['attributes']['supportsVoiceover'] = true
      assert(check(path, args)[:errors].any? { |error| error.include?('changed') })
      declarations.first['attributes']['state'] = 'DRAFT'
      assert(!check(path, args)[:ok])
    end
  end

  def test_candidate_regression_is_not_hidden_by_a_followup_plan
    with_review do |data, path, args, save|
      data['families']['IPAD']['capabilities']['supportsDarkInterface']['supported'] = false
      data['after_release'] = {'owner' => 'Reviewer', 'plan' => 'Fix later'}
      save.call
      result = check(path, args)
      assert(!result[:ok] && result[:errors].any? { |error| error.include?('IPAD supportsDarkInterface regresses') })
    end
  end

  def test_future_claims_require_followup_and_do_not_publish
    with_review do |data, path, args, save|
      data['families']['IPHONE']['capabilities']['supportsLargerText']['supported'] = true
      save.call
      assert(!check(path, args)[:ok])
      data['after_release'] = {'owner' => 'Reviewer', 'plan' => 'After exact version/build is live, publish verified claims and run live-status'}
      save.call
      result = check(path, args)
      assert(result[:ok] && result[:post_live_claims] == ['IPHONE.supportsLargerText'])
      assert(declarations.first['attributes']['supportsLargerText'] == false)
    end
  end

  def test_post_live_requires_exact_candidate_build_and_both_published_families
    with_review do |_data, path, args, _save|
      assert(!check(path, args, live: true)[:ok])
      @responses['/v1/apps/app/appStoreVersions?filter[platform]=IOS&limit=200']['data'].last['attributes']['versionString'] = '1.2.3'
      assert(!check(path, args, live: true)[:ok])
      @responses['/v1/appStoreVersions/live/build']['data']['attributes']['version'] = '26'
      assert(check(path, args, live: true)[:ok])
      declarations.last['attributes']['supportsDarkInterface'] = false
      assert(!check(path, args, live: true)[:ok])
      declarations.pop
      assert(!check(path, args, live: true)[:ok])
    end
  end

  def test_pagination_is_read_completely_and_duplicates_or_unknown_fields_are_rejected
    with_review do |_data, path, args, _save|
      first = @responses['/v1/apps/app/accessibilityDeclarations?filter[state]=PUBLISHED&limit=200']
      second = first['data'].pop
      first['links'] = {'next' => 'https://api.appstoreconnect.apple.com/v1/page-two'}
      @responses[first['links']['next']] = {'data' => [second]}
      assert(check(path, args)[:ok])
      first['data'] << second
      assert(!check(path, args)[:ok])
      first['data'].pop
      second['attributes']['supportsNewFeature'] = true
      assert(!check(path, args)[:ok])
      second['attributes'].delete('supportsNewFeature')
      @responses[first['links']['next']]['links'] = first['links']
      assert(!check(path, args)[:ok])
      first['links']['next'] = 'https://not-apple.example/v1/metadata'
      assert(!check(path, args)[:ok])
    end
  end

  def test_live_missing_build_first_release_and_api_failure_are_not_passes
    with_review do |_data, path, args, _save|
      @responses['/v1/appStoreVersions/live/build']['data'] = nil
      assert(!check(path, args)[:ok])
      @responses['/v1/apps/app/appStoreVersions?filter[platform]=IOS&limit=200']['data'].pop
      assert(!check(path, args)[:ok])
      @responses.clear
      assert(!check(path, args)[:ok])
    end
  end
end

AccessibilityTest.instance_methods.grep(/^test_/).sort.each do |name|
  AccessibilityTest.new.public_send(name)
  puts "passed #{name}"
end
